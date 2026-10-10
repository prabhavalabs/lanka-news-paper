# Builds the three release images in parallel from artifacts in .ci-out/.
variable "TAG" { default = "dev" }
variable "REGISTRY" { default = "ghcr.io/prabhavalabs/lanka-news-paper" }

group "default" { targets = ["api", "web", "admin"] }

target "_release" {
  context    = "."
  dockerfile = "infra/production/package.Dockerfile"
  platforms  = ["linux/amd64"]
}

target "api" {
  inherits   = ["_release"]
  target     = "api"
  tags       = ["${REGISTRY}-api:${TAG}", "${REGISTRY}-api:main"]
  cache-from = ["type=gha,scope=pkg-api"]
  cache-to   = ["type=gha,mode=max,scope=pkg-api"]
}

target "web" {
  inherits   = ["_release"]
  target     = "web"
  tags       = ["${REGISTRY}-web:${TAG}", "${REGISTRY}-web:main"]
  cache-from = ["type=gha,scope=pkg-frontend"]
  cache-to   = ["type=gha,mode=max,scope=pkg-frontend"]
}

target "admin" {
  inherits   = ["_release"]
  target     = "admin"
  tags       = ["${REGISTRY}-admin:${TAG}", "${REGISTRY}-admin:main"]
  cache-from = ["type=gha,scope=pkg-frontend"]
}
