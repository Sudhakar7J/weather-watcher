# Design notes

Some of the choices here weren't obvious, and a few were close calls. This is
where I write down what I went with and why, so you don't have to reverse
engineer it from the code. Where something is a real limitation I've said so
rather than pretending otherwise.

---

## The upstream: Open-Meteo, because it needs no key

WeatherAPI was the other option the brief mentioned, but it requires signing up
for an API key. That key then has to live somewhere — Secrets Manager, or a
GitHub Actions secret — and becomes something to rotate and something that can
leak in a CI log.

Open-Meteo needs no key at all. That meant zero credentials anywhere in the
system, which is a real reduction in attack surface rather than just one less
thing to set up. It also meant I could spend the time I saved not building a
secret plumbing layer on actually verifying the cache behaves correctly under
concurrency.

The trade-off: if Open-Meteo has an outage, every city fails at once, because
we only hold an hour of cached data and there's no second source to fall back
on. For a free API with no SLA that's an acceptable bet, but it's a bet.

---

## Caching: in memory, and I'm deliberately not caching failures

The brief allows in-memory, SQLite, or similar. I went in-memory because of
what happens on App Runner: the filesystem is ephemeral, so a SQLite file would
be wiped on every deploy anyway. You'd carry a schema and migrations for data
that's derived and free to re-fetch. Redis was the other option but it's a
network hop for something a single process can hold in a map.

The thing I want to flag is the failure case. If the upstream 500s, we do **not**
cache that error for the hour. Early on I was tempted to, because hammering a
struggling API feels rude — but caching a transient blip means one unlucky
request gives a user an hour of broken cards. Retrying costs one extra upstream
call. In a real system I'd probably add a much shorter negative TTL, a few
seconds; zero is the simple version of the right idea.

---

## The part I spent the most time on: concurrent misses

The brief asks for a 1-hour cache and separately grades "safe handling of
concurrency". Those turn out to be two different problems, and the obvious
implementation — a `map` behind a `sync.Mutex` — only solves the first.

Picture a cold cache and three browser tabs opening the dashboard at once, all
asking for Singapore. With a plain mutex-guarded map, all three miss, and all
three call Open-Meteo. The mutex protects the map from corruption; it does
nothing about the fact that you're now making three identical upstream requests.

So each key tracks whether a load is in flight. Misses on the same key join the
existing load and share its result — one upstream call, three satisfied
requests.

The trap I wanted to avoid is fixing it by holding the lock for the duration of
the load. That makes the concurrency test pass while quietly serialising the
entire service, so every city now blocks behind every other city. There's a test
specifically asserting that distinct keys still load in parallel, because the
naive fix would sail right past it.

I wrote the coalescing by hand rather than pulling in `golang.org/x/sync/singleflight`. That was partly a zero-dependency stance (below) and partly that it let me make it generic, which `singleflight` isn't. The honest cost is that this is code I own and therefore code I have to test properly — which is why the cache has 12 tests, more than it otherwise would.

One behaviour worth stating: a waiter whose request gets cancelled gives up
early but does *not* cancel the shared load. The load has other waiters, or it
was already paid for. Cancelling it would punish them for one client hanging up.

---

## Generic cache, no third-party dependencies

Two related decisions.

The cache is `Cache[V any]` using generics. It has exactly one consumer today,
so a cache hard-coded to `weather.Report` would have been less code. But then
the package would depend on the domain package and couldn't be tested on its
own, and a `map[string]any` with type assertions can panic at runtime. Generics
make the wrong type a compile error.

On dependencies: the backend uses the standard library only. Go 1.22 added
method-and-path patterns to `http.ServeMux`, so there's no need for a router,
and the coalescing is small enough to own. No `go.mod` dependencies means no
supply chain and nothing for CI to scan. I'd absolutely reach for a library
once there's a real need — structured logging, say — and I didn't want this to
be a rule so much as a default that happens to hold at this size.

---

## One bug worth writing down

The API response says whether each report came from cache. The obvious way is
to cache `*Report` and flip a `FromCache` field on it before returning.

That's a data race. Two requests reading the same cached report both write to
the same struct field, unsynchronised.

So reports are cached **by value**. The caller gets a copy and sets the field on
the copy; the shared entry is never touched. The one thing that *is* still
shared is the `Hourly` slice inside — safe only because nothing ever mutates it
after construction. That constraint is documented on the type, because it's
exactly the kind of thing a future contributor breaks without noticing.

---

## Per-city errors instead of one big failure

`?cities=Singapore,Tokyo,London` — what if one is misspelled?

The simplest API returns an error for the whole request. I did not, because one
typo in someone's tracked list would blank their entire dashboard. The response
is a 200 with a `results` array and a separate `errors` array, so the frontend
can show one bad card while everything else stays live. It costs a slightly
richer response type, and clients have to check both arrays.

---

## Infrastructure: App Runner

App Runner, ECS Fargate, or EC2 were all on the table.

App Runner wins because the brief specifically calls out that it gives you HTTPS
out of the box, and cross-origin HTTPS is exactly what the frontend needs. It
also scales to zero, which matters here — the brief asks us to destroy
infrastructure after review, and a service that's free while idle is one less
thing to think about.

Fargate would mean VPC, subnets, a load balancer, and a task definition — maybe
four times the Terraform for an identical outcome. EC2 is cheapest at low
traffic but needs an instance that patches itself and destroys itself
correctly on termination, which is the riskiest possible thing to leave lying
around in someone else's account.

What I give up: there's no VPC to drop the service into without extra work, and
App Runner bills per active instance, so a bursty traffic pattern would cost
more than Fargate. Neither matters at this scale.

---

## Terraform: local state, and that's a real compromise

Local state, because the brief allows it and because it means you can clone and
run `terraform init && terraform apply` with nothing to configure first. That
matters for a take-home someone else has to reproduce.

It is not concurrency-safe and there is no history if it's lost. If this were a
real service shared by a team, the first change would be an S3 backend with
state locking and versioning on the bucket. I've flagged that in the README too
rather than leaving it implied.

Related: `.terraform.lock.hcl` **is** committed while `*.tfstate` is not. The
lock file pins provider versions so every `init` resolves identically; state
files hold your actual infrastructure and stay local.

---

## CI: publish an image, don't apply infrastructure

The brief allows either `terraform apply` inside GitHub Actions, or applying
once and using CI only to publish the image.

I went with the second. Full apply in CI would give Actions broad AWS
permissions and would silently correct any drift on every push, and the
workflow gets much slower and harder to debug. The narrower version — build,
push to ECR, trigger an App Runner deployment — can't create or destroy
anything, so a bad workflow run has a small blast radius.

The cost is that drift is possible between `terraform apply` and reality. A
`terraform plan` step in CI would catch it and is the natural next addition.

---

## Frontend

**Hosted on Vercel.** The brief is explicit that Terraform isn't required for
the frontend and lists Vercel first. It gives HTTPS, a real domain, and CI
deploys from GitHub without any infrastructure. Hosting it on S3 + CloudFront
would keep everything in one cloud and look tidier, but it's meaningfully more
setup for no benefit the brief asks for. Happy to move it if you'd rather have
it all in AWS.

**Tracked cities in `localStorage`.** They survive a refresh, which is the
actual requirement. The downside is that they don't follow you between browsers
or devices, and there's no account system — the brief only asked for per-client
state, so I didn't build one.

**One `CityCard`, and the card owns its own states.** The brief calls this out
specifically, and it's the right instinct — a card that renders its own loading,
error, and loaded state means the parent never branches its markup around
individual cards. Adding a city is adding an array entry, not copying markup.

**In-flight requests get aborted when the tracked list changes.** Without this
there's a genuine race: you remove Singapore, the request for the old list is
still in flight, and when it lands it overwrites your fresh state. It's a small
thing that produces a bug that's miserable to reproduce.

---

## Limitations I know about

Listing these because I'd rather say them than have you find them.

- **The cache is per-instance.** If App Runner ever runs more than one instance,
  each keeps its own cache and the upstream hit rate drops proportionally. The
  fix is a shared store.
- **No rate limiting.** A public deployment would want a per-IP cap so the API
  can't be used as a free proxy for Open-Meteo.
- **The API is unauthenticated.** Fine for public weather data, not fine if this
  ever held anything else.
- **Local Terraform state.** Covered above.
