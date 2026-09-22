# Modular Go Lambda Starter

A reusable modular-monolith starter for server-rendered Go applications using Turso, chi, HTML templates, HTMX and custom CSS. The same binary runs as a local HTTP server or as an AWS Lambda container image behind API Gateway HTTP API.

## Stack

- Go 1.25
- `github.com/go-chi/chi/v5` for composable routing and standard `net/http` handlers
- `github.com/akrylysov/algnhsa` for a lightweight Lambda/API Gateway `net/http` adapter
- `turso.tech/database/tursogo-serverless` for pure-Go remote Turso access over HTTP
- `html/template` with `go:embed` for startup-parsed server-side rendering
- HTMX for incremental HTML updates
- Custom CSS embedded into the binary
- AWS Lambda `provided.al2023` container image
- Amazon ECR and GitHub Actions for deployment

## Project Shape

```text
.
|- cmd/app/main.go                 # local HTTP and Lambda entry point
|- internal/app/app.go             # composition root
|- internal/modules/
|  |- health/                      # health and readiness endpoints
|  |- home/                        # overview page
|  `- todos/                       # example vertical feature slice
|- internal/platform/
|  |- config/                      # environment configuration
|  |- database/                    # Turso connector and pool settings
|  |- httpserver/                  # router and shared middleware
|  `- web/                         # embedded templates and static assets
|     |- templates/                # HTML layouts, pages and partials
|     `- static/css/app.css        # custom UI styles
|- Dockerfile                      # multi-stage Lambda image
`- .github/workflows/deploy.yml    # ECR push and Lambda update
```

New features should normally follow this shape:

```text
internal/modules/orders/
|- orders.go
|- repository.go
`- ...
internal/platform/web/templates/pages/orders.html
internal/platform/web/templates/partials/order-row.html
```

Register the module from `internal/app/app.go` by passing its `Routes` method to `httpserver.NewRouter`.

## Local Setup

1. Replace `example.com/go-lambda-monolith` in `go.mod` and the Go imports with your repository module path.
2. Copy `.env.example` to `.env`. Local startup loads `.env`; shell variables always take precedence.
3. Set `TURSO_DATABASE_URL` to your `libsql://...` or `https://...` database URL.
4. Set `TURSO_AUTH_TOKEN` to a database token. Do not commit it.
5. Start the app with `go run ./cmd/app`.
6. Open `http://localhost:8080`.

The application runs the example todos migration at startup. The migration is idempotent and can later be moved into a dedicated migration runner when the project has multiple schema versions.

## Template Decision

This starter uses Go's `html/template` rather than `templ`. `templ` generates Go code and can win on render-loop benchmarks, but it also adds a code-generation toolchain to every template change. Here templates are parsed once during application initialization, then reused concurrently, keeping the Lambda image and local workflow smaller. The renderer is isolated in `internal/platform/web`, so a CPU-bound project can replace it with generated `templ` components without changing feature route boundaries.

## Turso And Lambda Notes

This starter uses the serverless Turso driver because Lambda is stateless and does not provide a durable local filesystem. The driver uses `database/sql` and a connector so the auth token never appears in a DSN. The database pool is created without an eager ping; the idempotent startup migration performs the first required request, reducing cold-start network work.

The default pool is intentionally conservative:

- `DB_MAX_OPEN_CONNS=1`
- `DB_MAX_IDLE_CONNS=1`
- `DB_CONN_MAX_LIFETIME=5m`

Increase these only after measuring your Lambda concurrency and Turso limits. Each warm Lambda execution environment owns its own pool.

Configure these environment variables on the Lambda function:

```text
TURSO_DATABASE_URL=libsql://your-database.turso.io
TURSO_AUTH_TOKEN=your-database-token
HTTP_REQUEST_TIMEOUT=10s
DB_MAX_OPEN_CONNS=1
DB_MAX_IDLE_CONNS=1
DB_CONN_MAX_LIFETIME=5m
```

The application reads environment variables only. If Secrets Manager or SSM Parameter Store is your source of truth, inject those values during infrastructure deployment or add a secrets loader before `app.New`.

The deployment workflow does not put Turso credentials into the Docker image or GitHub Actions. Configure the Turso variables directly on the Lambda function.

## AWS Prerequisites

Create the ECR repository and Lambda function manually in the same AWS region before deployment. The first GitHub Actions run only builds and pushes the image to ECR. If the Lambda function does not exist yet, the workflow prints the pushed image URI and exits successfully. Create the Lambda function from that image, then later GitHub Actions runs update the same function identified by `LAMBDA_FUNCTION_NAME`.

Set these GitHub repository secrets:

```text
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
```

Add these in GitHub at `Settings -> Secrets and variables -> Actions -> Secrets -> New repository secret`.

Configure these environment variables manually in the Lambda console under `Configuration -> Environment variables`:

```text
APP_ENV=production
HTTP_ADDR=:8080
HTTP_REQUEST_TIMEOUT=10s
TURSO_DATABASE_URL=libsql://your-database.turso.io
TURSO_AUTH_TOKEN=your-database-token
DB_MAX_OPEN_CONNS=1
DB_MAX_IDLE_CONNS=1
DB_CONN_MAX_LIFETIME=5m
```

Optional GitHub repository variables:

```text
AWS_REGION=ap-south-1
ECR_REPOSITORY=go-lambda-monolith
LAMBDA_FUNCTION_NAME=go-lambda-monolith
```

These are optional GitHub repository variables. Add them at `Settings -> Secrets and variables -> Actions -> Variables -> New repository variable`. Defaults are `ap-south-1`, `go-lambda-monolith`, and `go-lambda-monolith`.

The workflow uses `linux/amd64`. If the Lambda function is configured for ARM64, update both the workflow platform and the Lambda architecture together.

### First Deployment

1. Create the ECR repository manually in `ap-south-1`.
2. Add only `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` to GitHub Actions Secrets.
3. Add optional repository variables if the defaults are not suitable.
4. Push to `main` or run the workflow manually. The image is pushed to ECR and the workflow prints the image URI because the Lambda function does not exist yet.
5. In AWS Lambda, choose `Create function -> Container image`.
6. Use the exact `LAMBDA_FUNCTION_NAME` value and the image URI printed by Actions.
7. Use `x86_64` architecture because the workflow builds `linux/amd64`.
8. Add the Turso and application environment variables shown above in the Lambda configuration.
9. After this, every push to `main` publishes a new ECR image and updates the same Lambda function.

The image URI format is:

```text
ACCOUNT_ID.dkr.ecr.ap-south-1.amazonaws.com/ECR_REPOSITORY:IMAGE_TAG
```

## Performance Choices

- `html/template` files are embedded and parsed once during initialization, not per request.
- The pure-Go Turso serverless driver avoids CGO and native runtime dependencies.
- The image uses a multi-stage build, `lambda.norpc`, `-trimpath` and stripped linker flags.
- Static assets are embedded, so the deployed image is self-contained.
- chi and standard `net/http` keep routing and middleware overhead small.
- HTMX requests return only the affected HTML partial rather than a full document.
- The Lambda workflow uses BuildKit layer caching and immutable run tags.

## Important Deployment Detail

Lambda resolves a container tag to an image digest. Pushing a new image tag alone does not update an existing Lambda function. The workflow correctly calls `aws lambda update-function-code` with a unique immutable run tag after every image push.

## Reference

The Turso connection approach follows the official Go documentation: <https://docs.turso.tech/connect/go>. For this Lambda target, the remote `tursogo-serverless` driver is used instead of the local embedded `tursogo` driver because Lambda execution environments are stateless and the application needs direct remote access to Turso.
