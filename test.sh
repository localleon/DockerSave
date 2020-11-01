rm -rf ./layers/*

export AUTH_SERVICE='registry.docker.io'
export AUTH_SCOPE="repository:alpine/git:pull"
export REGISTRY_URL="https://registry-1.docker.io"

export TOKEN=$(curl -fsSL "https://auth.docker.io/token?service=$AUTH_SERVICE&scope=$AUTH_SCOPE" | jq --raw-output '.token')

curl --output "./layers/infos.json" -fsSL -H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" -H "Authorization: Bearer $TOKEN" "$REGISTRY_URL/v2/alpine/git/manifests/latest" | jq

# Also Possible
#Accept: application/vnd.docker.distribution.manifest.v2+json
#Accept: application/vnd.docker.distribution.manifest.list.v2+json
#Accept: application/vnd.docker.distribution.manifest.v1+json

# Get Manifest Blobl from infos.json
#Request /v2/alpine/git/manifests/sha256:HASHFROMinfos.JSON
export MANIFEST="sha256:8715680f27333935bb384a678256faf8e8832a5f2a0d4a00c9d481111c5a29c0"
curl --output "./layers/manifest.json" -fsSL -H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" -H "Authorization: Bearer $TOKEN" "$REGISTRY_URL/v2/alpine/git/manifests/$MANIFEST" | jq

# GET Config Blob with ID from manifest
export CONFIG1="sha256:a8b6c5c0eb622fe4252b425dce65ac9117ddd45103116b6ca05fe6196bfa97b8"
curl -L --output "./layers/${CONFIG1}.json" -H "Authorization: Bearer $TOKEN" "$REGISTRY_URL/v2/alpine/git/blobs/$CONFIG1" 

# # Get Blob for each Layer ID from manifest
export LAYER1="sha256:df20fa9351a15782c64e6dddb2d4a6f50bf6d3688060a34c4014b0d9a752eb4c"
export LAYER2="sha256:c04c7f41a94db41804e424c4bada807f753c586f1b6da4febe38393f5ea85c82"
export LAYER3="sha256:cfb833d2ca17198ee2ed5de3805d63fa42ae5af255e89de4daae1fcf04078527"

curl -L --output "./layers/${LAYER1}" -H "Authorization: Bearer $TOKEN" "$REGISTRY_URL/v2/alpine/git/blobs/$LAYER1" 
curl -L --output "./layers/${LAYER2}" -H "Authorization: Bearer $TOKEN" "$REGISTRY_URL/v2/alpine/git/blobs/$LAYER2" 
curl -L --output "./layers/${LAYER3}" -H "Authorization: Bearer $TOKEN" "$REGISTRY_URL/v2/alpine/git/blobs/$LAYER3" 



# MAybe imitate this https://github.com/NotGlop/docker-drag/blob/master/docker_pull.py