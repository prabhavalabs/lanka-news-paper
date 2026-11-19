# Builds the three release images in parallel from artifacts in .ci-out/.
# The layer cache lives in the registry (":buildcache" tags), so it survives gaps between
# deploys and the heavy API runtime layer (Chromium, Codex CLI) is only rebuilt when it changes.
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
  tags       = ["${REGISTRY}-api:${TAG}"]
  cache-from = ["type=registry,ref=${REGISTRY}-api:buildcache"]
  cache-to   = ["type=registry,ref=${REGISTRY}-api:buildcache,mode=max"]
}

target "web" {
  inherits   = ["_release"]
  target     = "web"
  tags       = ["${REGISTRY}-web:${TAG}"]
  cache-from = ["type=registry,ref=${REGISTRY}-web:buildcache"]
  cache-to   = ["type=registry,ref=${REGISTRY}-web:buildcache,mode=max"]
}

target "admin" {
  inherits   = ["_release"]
  target     = "admin"
  tags       = ["${REGISTRY}-admin:${TAG}"]
  cache-from = ["type=registry,ref=${REGISTRY}-web:buildcache"]
}
