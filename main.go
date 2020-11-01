package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

var authService string = "registry.docker.io"
var registryURL string = "https://registry-1.docker.io"
var api *http.Client
var authResult AuthToken

func main() {
	api = &http.Client{}

	image := "alpine/git"
	tag := "latest"
	authScope := "repository:" + image + ":pull"

	fmt.Println("Getting Authenticaton Token")
	// Get Authentication Token for authScope
	uri := "https://auth.docker.io/token?service=" + authService + "&scope=" + authScope
	resp, err := http.Get(uri)
	if err != nil {
		fmt.Println(err)
	}

	json.NewDecoder(resp.Body).Decode(&authResult)

	fmt.Println("Retrieving Information about Container Image")
	infos := getManifestInfos(image, tag)
	amd64 := infos.Manifests[0]

	// Download Manifest
	fmt.Println("Downloading Manifest Files")
	manifest := downloadManifest(image, amd64.Digest)
	writeManifestToFile(manifest, "./golayer/manifest.json")

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
