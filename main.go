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
	"time"
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
	out := "alpine.tar"

	fmt.Println("Downloading " + image + ":" + tag)

	// Get Authentication Token for authScope
	fmt.Println("Getting Authenticaton Token for " + authService)
	authResult = getAuthToken(authService, authScope)

	// Getting Information
	fmt.Println("Retrieving Information about Container Image on " + registryURL)
	infos := getManifestInfos(image, tag)
	amd64 := infos.Manifests[0]

	// Download Manifest
	fmt.Println("Downloading Manifest Files")
	fmt.Println("----------------------------------------------------")
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
	v := ""
	var parentID *string
	parentID = &v

	for i, l := range manifest.Layers {
		blob := l.Digest
		fmt.Println("Pulling Layer Blob: ", blob[7:30])

		// Create first Layer Folder#
		fakeLayerID := generateFakeID(*parentID, blob)
		_ = os.Mkdir("./golayer/"+fakeLayerID, 0777)
		downloadLayerBlob(image, blob, fakeLayerID)

		// Create Version File
		f, _ := os.Create("./golayer/" + fakeLayerID + "/VERSION")
		f.Write([]byte("1.0"))
		f.Close()

		// Append Layer ID to Manifest File
		contentManifest.Layers = append(contentManifest.Layers, fakeLayerID)

		// Create JSON File for Layer (from emptyJson)

		if len(manifest.Layers)-1 == i { // Check if it's the last Layer
			createJSONLastLayerFile(parentID, fakeLayerID, config)
		} else {
			createJSONLayerFile(parentID, fakeLayerID, config)
		}
	}

	// Write Image Content Manifest to File
	contentFile, _ := json.MarshalIndent(contentManifest, "", " ")
	_ = ioutil.WriteFile("./golayer/manifest.json", contentFile, 0644)

	// Create Repo File
	createRepoFile(image, *parentID)
	fmt.Println("\n \nFinished pulling " + image + ":" + tag)
	fmt.Println(out)
	// Create archive
	// fmt.Println("Creating Tarball out of layers.....")
	// err2 := Tar("golayer", "./")
	// if err != nil {
	// 	fmt.Println(err2.Error())
	// }
	// os.RemoveAll("./golayer/")
	// os.Rename("./golayer.tar", out)
}

func createJSONLastLayerFile(parentID *string, fakeLayerID string, c ImageConfig) {
	// delete history object in it
	c.History = []struct {
		Created    time.Time "json:\"created\""
		CreatedBy  string    "json:\"created_by\""
		EmptyLayer bool      "json:\"empty_layer,omitempty\""
	}{}
	// delte rootfs object
	c.Rootfs = struct {
		Type    string   "json:\"type\""
		DiffIds []string "json:\"diff_ids\""
	}{}
	// set ID
	c.ID = fakeLayerID
	// set parentID
	c.Parent = *parentID
	*parentID = fakeLayerID
	// Write JSON to file
	jsonFile, _ := json.MarshalIndent(c, "", " ")
	err := ioutil.WriteFile("./golayer/"+fakeLayerID+"/json", jsonFile, 0777)
	if err != nil {
		println(err.Error())
	}
}

// BUg parentID is not set
func createJSONLayerFile(parentID *string, fakeLayerID string, c ImageConfig) {
	var jsonData LayerJson
	jsonData.Created = c.Created // Set Creation date
	jsonData.ID = fakeLayerID
	if *parentID != "" {
		jsonData.Parent = *parentID // set reference to last object
	}
	*parentID = fakeLayerID // set current ID for next iteration process
	// Write JSON to file
	jsonFile, _ := json.MarshalIndent(jsonData, "", " ")
	err := ioutil.WriteFile("./golayer/"+fakeLayerID+"/json", jsonFile, 0777)
	if err != nil {
		println(err.Error())
	}
}

func createRepoFile(image, digest string) {
	// Write Repository File
	repo := map[string]interface{}{
		image: struct {
			Latest string
		}{digest},
	}
	repoFile, _ := json.MarshalIndent(repo, "", " ")
	_ = ioutil.WriteFile("./golayer/repositories", repoFile, 0644)
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
