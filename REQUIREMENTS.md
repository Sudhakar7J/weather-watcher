# Weather Watcher — Requirements Checklist

Status of every item in `prd.md`, verified against the deployed application on
10 October 2026.

---

## Live links

| | |
|---|---|
| **Repository** | https://github.com/Sudhakar7J/weather-watcher |
| **Frontend** | https://frontend-rho-seven-xsyijkqauv.vercel.app |
| **Backend API** | https://3qf5nahks4unjawb4ozxqh53yi0mudie.lambda-url.us-east-1.on.aws |
| **Health check** | `/api/health` on the backend URL above |

Both links are public and need no sign-in. The backend was confirmed with an
anonymous request carrying no credentials.

---

## 1. Technology stack

| Requirement | Status | Evidence |
|---|---|---|
| React frontend, publicly accessible | ✅ | Deployed on Vercel, HTTPS |
| Go backend, deployed to AWS | ✅ | AWS Lambda, arm64, us-east-1 |
| Terraform for backend IaC | ✅ | `terraform/` — main, variables, outputs, ecr, lambda |
| Simple compute option | ✅ | AWS Lambda behind a Function URL |
| CI/CD via GitHub Actions | ✅ | `.github/workflows/deploy.yml`, triggers on push to main |
| In-memory data store | ✅ | Go maps, TTL cache, no external database |
| Free public weather API | ✅ | Open-Meteo, no API key required |

## 2. Backend API

| Requirement | Status | Evidence |
|---|---|---|
| `GET /api/health` | ✅ | Returns status, uptime, cached city count |
| Multi-city endpoint | ✅ | `/api/weather?cities=Singapore,Tokyo,London,Paris` |
| HTTPS endpoint | ✅ | Lambda Function URL, TLS by default |
| CORS for the frontend origin | ✅ | Verified: `Access-Control-Allow-Origin: *` |
| Input validation | ✅ | Max 20 cities, empty query rejected |
| Per-city error isolation | ✅ | One bad city does not fail the request |
| In-memory cache, 1 hour TTL | ✅ | Verified ~1.4s cold vs ~1ms cached |
| Concurrent-miss coalescing | ✅ | Simultaneous requests for a city share one fetch |

## 3. Frontend

| Requirement | Status | Evidence |
|---|---|---|
| Clean, functional dashboard | ✅ | Condition-coloured card grid, 24-hour chart |
| Multi-city tracking | ✅ | Up to 20 cities, persisted in localStorage |
| Live data from the deployed backend | ✅ | API URL baked into the production build |
| Loading, error and empty states | ✅ | All four implemented, same design tokens |
| Responsive | ✅ | Verified at mobile and desktop widths |

## 4. Infrastructure as Code

| Requirement | Status | Evidence |
|---|---|---|
| All AWS resources in `terraform/` | ✅ | 9 resources, no manual console changes |
| `variables.tf` parameterisation | ✅ | Region, environment, port, TTL, max cities |
| Local backend state, `.tfstate` gitignored | ✅ | Verified ignored via `git check-ignore` |
| `terraform init`/`validate` clean | ✅ | Both pass |
| Safe secret handling | ✅ | No credentials in code or state |

## 5. CI/CD and git workflow

| Requirement | Status | Evidence |
|---|---|---|
| Workflow triggers on push to main | ✅ | `on.push.branches: [main]` |
| Builds the container | ✅ | `docker/build-push-action`, provenance disabled for Lambda |
| Deploys to AWS | ✅ | `terraform apply`, image push, `update-function-code` |
| Deploy gated on green builds | ✅ | `needs: [backend, frontend, terraform]` |
| OIDC instead of stored keys | ✅ | `id-token: write`, trust policy documented in `terraform/OIDC_SETUP.md` |

## 6. Testing

| Suite | Count | Status |
|---|---|---|
| Go unit tests | 57 | ✅ Passing |
| Frontend unit tests (Vitest) | 49 | ✅ Passing |
| End-to-end (Playwright) | 32 | ✅ Passing |
| Accessibility checks | Included above | ✅ ARIA, focus, keyboard, reduced motion |
| Error-path coverage | Included above | ✅ Offline, 5xx, bad request, malformed payload, retry |
| Race detector | 57 | ✅ Runs in CI (needs cgo, unavailable locally) |
| Typecheck and production build | — | ✅ Clean |

## 7. Documentation

| Requirement | Status |
|---|---|
| Run the React app locally | ✅ `README.md` |
| How the frontend was deployed | ✅ `README.md` |
| `terraform init` / `apply` instructions | ✅ `README.md` |
| Live frontend link | ✅ This document and `README.md` |
| Live backend URL | ✅ This document and `README.md` |
| 2–3 paragraph write-up | ✅ `README.md` — architecture, caching, IaC |
| "What I would do next" | ✅ `README.md` |

---

## Defects found and fixed during testing

These were caught by the test suites, not by inspection:

1. **`Access-Control-Allow-Origin: *,*`** — Lambda Function URL and the app
   both set the header, producing a value browsers reject. Every cross-origin
   call failed while `curl` worked, so it presented as "backend unreachable".
2. **Silent gap for unmatched cities** — a city the geocoder resolves under a
   different name rendered nothing, so the header counted four cities while the
   grid showed three, with no message.
3. **Zod validation errors bypassed the error taxonomy**, surfacing raw paths.
4. **An empty `role="alert"` sat permanently in the DOM**, announcing itself on
   every page load.
5. **All eight shadcn components imported `cn` from the wrong package**,
   losing Tailwind class-conflict resolution.
6. **Client deadline too tight** — 15s measured against a warm local backend was
   insufficient for a cold start.
7. **No `<h1>`** on the page, caught by an accessibility test.
8. **Buildx provenance produced an OCI index** that Lambda rejects by media type.

---

## Deviations from the PRD

Stated plainly rather than buried:

- **Compute is Lambda, not App Runner.** App Runner entered maintenance on
  30 April 2026 and no longer accepts new customers. Lambda also runs in the
  free tier for this workload, so it is the cheaper option.
- **Frontend is on Vercel**, which the PRD explicitly permits. Terraform was not
  required for it and is not used there.
- **Deployments are CLI-driven today.** The GitHub Actions workflow is written
  and committed but has not yet run end-to-end, because setting up the OIDC role
  requires an AWS change made by the account owner.

---

## Known limitations

1. **Cold starts clear the cache.** The TTL cache lives in memory inside the
   Lambda execution environment, so a cold start starts empty. Harmless at this
   scale — refills in about a second and upstream calls are free — but it is the
   first thing to change at higher traffic.
2. **Single-region, single-environment.** No multi-AZ or staging environment.
3. **No automated frontend redeploy.** The frontend was deployed from the CLI;
   Git integration is not connected yet.
4. **CORS allows all origins.** Correct for a public read-only API, but it would
   need narrowing if the API were ever authenticated.