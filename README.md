# Weather Watcher

A multi-city weather dashboard. A Go API proxies and caches live data from
[Open-Meteo](https://open-meteo.com) — no API key anywhere in the stack — and a
React frontend renders it.

| | |
|---|---|
| **Frontend** | https://frontend-rho-seven-xsyijkqauv.vercel.app |
| **Backend API** | https://3qf5nahks4unjawb4ozxqh53yi0mudie.lambda-url.us-east-1.on.aws |
| **Health check** | https://3qf5nahks4unjawb4ozxqh53yi0mudie.lambda-url.us-east-1.on.aws/api/health |
| **Repository** | https://github.com/Sudhakar7J/weather-watcher |
| **Requirements** | See [REQUIREMENTS.md](./REQUIREMENTS.md) for the full checklist |

Both links are public and need no sign-in.

---

## Architecture

The backend is three small layers composed once, in `cmd/server` and
`cmd/lambda`: an Open-Meteo client that knows the upstream wire format, a cache
that decides whether to call it, and a service that joins the two and returns a
domain-shaped `Report`. HTTP handling sits on top as a plain `net/http.Handler`
built from the request's method and path patterns. That last detail is what lets
one codebase serve two very different runtimes. `cmd/server` is the conventional
long-running process on port 8080. `cmd/lambda` is a roughly hundred-line
adapter that translates a Function URL event into an `*http.Request`, hands it to
the *same* handler, and writes the response back. Every behavioural fix
therefore applies to both entrypoints at once — there is no second
implementation to keep in sync.

Caching is an in-memory `map` with a one-hour TTL, guarded by a mutex. Two
details matter more than the TTL itself. First, concurrent misses for the same
city share a single upstream fetch through per-key in-flight tracking, so ten
simultaneous requests for Singapore produce one call to Open-Meteo rather than
ten; without this, a cold dashboard would fan out and most of those calls would
return the same answer. Second, errors are never cached, so a transient upstream
failure cannot pin a city in a failed state for an hour. The cache also stores
values rather than pointers, which removes a whole class of aliasing bugs when a
cached report is read concurrently. `fetched_at` and `from_cache` are carried on
the wire for observability, though the UI deliberately does not surface a
"cached" badge — it is an implementation detail rather than something a user
needs.

Infrastructure is Terraform across nine resources in us-east-1: a private ECR
repository with scan-on-push, a Lambda function running arm64 from that
repository, a Function URL for the public HTTPS endpoint, an explicit log group
with 14-day retention so retention is enforced from the first invocation, and
least-privilege IAM roles. The Lambda execution role can pull only this one
repository and write only to its own log group. State is a local backend and
`.tfstate` is gitignored, which the brief explicitly permits and which avoids
bootstrapping S3 and DynamoDB before the first apply. Two decisions are worth
calling out. The image is built with `--provenance=false`, because the buildx
default emits an OCI index with attestation manifests and Lambda rejects that
media type outright. And CORS is owned by exactly one layer: the Function URL
applies it at the edge, and the app skips its own headers when the Lambda trace
header is present. When both layers set the header independently the values
merged into `Access-Control-Allow-Origin: *,*`, which browsers reject — the
symptom was a dashboard reporting an unreachable backend while `curl` succeeded.

## Running locally

**Prerequisites:** Go 1.26+, Node 22+.

```bash
# Backend — http://localhost:8080
cd backend
go run ./cmd/server

# Frontend — http://localhost:5173
cd frontend
npm install
npm run dev
```

The frontend reads `VITE_API_BASE_URL` from `.env.local`; it defaults to
`http://localhost:8080`. Copy `.env.example` to get started.

## Tests

```bash
cd backend  && go test ./...          # 57 tests
cd frontend && npx vitest run         # 49 tests
cd frontend && npx playwright test    # 32 tests, needs the backend running
```

The Playwright suite runs against the real Go server rather than a stub, so
start the backend first. It covers behaviour, accessibility, responsive layout
and each failure path: offline, 5xx, bad request, malformed payload, and retry
recovery. The race detector runs in CI, where cgo is available.

## Deploying

**Frontend (Vercel).** The production build inlines the backend URL, so it must
be set before building:

```bash
cd frontend
npx vercel env add VITE_API_BASE_URL production
npm run build
vercel deploy --prod
```

**Backend (Terraform).**

```bash
cd terraform
terraform init
terraform plan
terraform apply
```

Then build and publish the image:

```bash
cd ../backend
docker buildx build --platform linux/arm64 --target lambda \
  --build-arg TARGET=lambda --provenance=false \
  -t <account>.dkr.ecr.us-east-1.amazonaws.com/weather-watcher-dev:latest --push .

aws lambda update-function-code --function-name weather-watcher-dev-api \
  --image-uri <account>.dkr.ecr.us-east-1.amazonaws.com/weather-watcher-dev:latest
```

CI does all of the above on a push to `main`, gated on green builds. It needs
`AWS_DEPLOY_ROLE_ARN` as a repository secret; `terraform/OIDC_SETUP.md` has the
trust policy, pinned to this repository and environment.

To remove everything and stop billing:

```bash
cd terraform && terraform destroy
```

## What I would do next

The most useful next step is moving the cache out of the process. It currently
lives in Lambda's memory, so a cold start starts empty — harmless at this
traffic, but it means the cache only ever helps within a single warm execution
environment. Redis or DynamoDB with a TTL would make it shared and durable.

After that, Git integration for the frontend, so pushes redeploy automatically
instead of via the CLI; and a staging environment so the deploy path is
exercised somewhere other than production.

One security item worth acting on soon: the AWS credentials used during this
build were long-lived IAM user access keys. The pipeline is already configured
for OIDC, which is the safer route, so rotating those keys and relying on OIDC
alone would close the gap.

## Known limitations

- Cold starts clear the in-memory cache; see above.
- CORS allows all origins, correct for a public read-only API but needing
  narrowing if it were ever authenticated.
- Single region, no staging environment.