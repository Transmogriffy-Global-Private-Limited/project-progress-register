[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
& go test .\api\openapi\v1 -run '^TestOpenAPIContract$' -count=1
if ($LASTEXITCODE -ne 0) { throw 'OpenAPI validation failed' }
