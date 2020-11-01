package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

var authService string = "registry.docker.io"
var registryURL string = "https://registry-1.docker.io"
var api *http.Client
var authResult AuthToken

func main() {
	api = &http.Client{}

	// Define default values for image download
	image := "alpine/git"
	tag := "latest"
	authScope := "repository:" + image + ":pull"

	// Get Authentication Token for authScope
	fmt.Println("Getting Authenticaton Token")
	authResult = getAuthToken(authService, authScope)

	// Getting Information
	fmt.Println("Retrieving Information about Container Image")
	infos := getManifestInfos(image, tag)
	amd64 := infos.Manifests[0]

	// Download Manifest
	fmt.Println("Downloading Manifest Files")
	manifest := downloadManifest(image, amd64.Digest)
	// writeManifestToFile(manifest, "./golayer/manifest.json")

	// Create Image Folder
	err := os.Mkdir("./golayer/", 0777)
	if err != nil {
		fmt.Printf(err.Error())
	}

	// Downloading Config Json
	config := downloadConfig(image, manifest.Config.Digest)
	configFile, _ := json.MarshalIndent(config, "", " ")
	configFileName := manifest.Config.Digest[7:] + ".json"
	_ = ioutil.WriteFile("./golayer/"+configFileName, configFile, 0644)

	// Build Content Manifest File
	var contentManifest ContentManifest
	contentManifest.Config = configFileName
	contentManifest.RepoTags = []string{image + ":" + tag}

	// Build Layers
	var parentID string

	l := manifest.Layers[0]
	blob := l.Digest
	// Create first Layer Folder
	fakeLayerID := generateFakeID(parentID, blob)
	_ = os.Mkdir("./golayer/"+fakeLayerID, 0777)
	downloadLayerBlob(image, blob, fakeLayerID)

	// Create Version File
	f, err := os.Create("./golayer/" + fakeLayerID + "/VERSION")
	f.Write([]byte("1.0"))
	f.Close()

	// Create JSON File for Layer
	var json LayerJson
	json.Created = config.Created // Set Creation date
	json.ID = fakeLayerID

}

func downloadLayerBlob(image, blob, fakeLayerID string) {
	uri := registryURL + "/v2/" + image + "/blobs/" + blob
	reqInfo, err := http.NewRequest("GET", uri, nil)
	reqInfo.Header.Add("Authorization", "Bearer "+authResult.Token)
	reqInfo.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")
	resp, err := api.Do(reqInfo)
	if err != nil {
		fmt.Println(err)
	}

	out, err := os.Create("./golayer/" + fakeLayerID + "/layer.tar")
	if err != nil {
		fmt.Println(err)
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	fmt.Println(err)
}

func generateFakeID(id, b string) string {
	s := id + "\n" + b + "\n"
	h := hmac.New(sha256.New, []byte("secret"))
	h.Write([]byte(s))

	return hex.EncodeToString(h.Sum(nil))
}

func downloadConfig(image, digest string) ImageConfig {
	uri := registryURL + "/v2/" + image + "/blobs/" + digest
	reqInfo, err := http.NewRequest("GET", uri, nil)
	reqInfo.Header.Add("Authorization", "Bearer "+authResult.Token)
	reqInfo.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")
	resp, err := api.Do(reqInfo)
	if err != nil {
		fmt.Println(err)
	}
	var result ImageConfig
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func writeManifestToFile(m Manifest, path string) {
	maniFile, _ := json.MarshalIndent(m, "", " ")
	_ = ioutil.WriteFile(path, maniFile, 0644)
}

// downloadManifest gets the manifest for a given image and plattform digest
func downloadManifest(image, digest string) Manifest {
	uri := registryURL + "/v2/" + image + "/manifests/" + digest
	reqInfo, err := http.NewRequest("GET", uri, nil)
	reqInfo.Header.Add("Authorization", "Bearer "+authResult.Token)
	reqInfo.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")
	resp, err := api.Do(reqInfo)
	if err != nil {
		fmt.Println(err)
	}
	var result Manifest
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func getManifestInfos(image, tag string) ManifestInfo {
	// Get Download Infos about blob storage
	uri := registryURL + "/v2/" + image + "/manifests/" + tag
	reqInfo, err := http.NewRequest("GET", uri, nil)
	reqInfo.Header.Add("Authorization", "Bearer "+authResult.Token)
	reqInfo.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")
	resp, err := api.Do(reqInfo)
	if err != nil {
		fmt.Println(err)
	}
	// Decode Manifest infos
	var result ManifestInfo
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func getAuthToken(service, scope string) AuthToken {
	uri := "https://auth.docker.io/token?service=" + service + "&scope=" + scope
	resp, err := http.Get(uri)
	if err != nil {
		fmt.Println(err)
	}

	var result AuthToken
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}
