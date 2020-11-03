package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

// We currently only support dockerhub
var authService string = "registry.docker.io"
var registryURL string = "https://registry-1.docker.io"

// Global Vars
var api *http.Client
var authResult AuthToken
var imgFlag string
var tagFlag string

func init() {
	flag.StringVar(&imgFlag, "image", "", "Specify the image to download. exp: alpine/git")
	flag.StringVar(&tagFlag, "tag", "", "Specify the tag to download. exp: latest or 1.7.1")
	flag.Parse()
}

func main() {
	api = &http.Client{}

	if !checkValidInput() {
		fmt.Println("Invalid Input. Please check your values. Aborting...")
		return
	}

	// Define default values for image download
	authScope := "repository:" + imgFlag + ":pull"
	fmt.Println("Downloading " + imgFlag + ":" + tagFlag)

	// Get Authentication Token for authScope
	fmt.Println("Getting Authenticaton Token for " + authService)
	authResult = getAuthToken(authService, authScope)

	// Getting Information
	fmt.Println("Retrieving Information about Container Image on " + registryURL)
	infos := getManifestInfos(imgFlag, tagFlag)
	amd64 := infos.Manifests[0]

	// Download Manifest
	fmt.Println("Downloading Manifest Files")
	fmt.Println("----------------------------------------------------")
	manifest := downloadManifest(imgFlag, amd64.Digest)

	// Create Image Folder
	err := os.Mkdir("./golayer/", 0777)
	if err != nil {
		fmt.Printf(err.Error())
	}

	// Downloading Config Json
	config, cfgErr := downloadConfig(imgFlag, manifest.Config.Digest)
	if cfgErr != nil {
		fmt.Println("Error while downloading Config.JSON. Aborting... ErrMsg:", cfgErr.Error())
	}
	configFile, _ := json.MarshalIndent(config, "", " ")
	configFileName := manifest.Config.Digest[7:] + ".json"
	cfgFErr := ioutil.WriteFile("./golayer/"+configFileName, configFile, 0644)
	if cfgFErr != nil {
		fmt.Println("Cant write config.json. Aborting.... ErrMSG: ", cfgFErr.Error())
	}

	// Build Content Manifest File
	var contentManifest ContentManifest
	contentManifest.Config = configFileName
	contentManifest.RepoTags = []string{imgFlag + ":" + tagFlag}

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
		layErr := downloadLayerBlob(imgFlag, blob, fakeLayerID)
		if layErr != nil {
			fmt.Println("Error while downloading Layer Blob. Aborting.... ErrMsg: ", layErr.Error())
		}

		// Create Version File
		f, _ := os.Create("./golayer/" + fakeLayerID + "/VERSION")
		f.Write([]byte("1.0"))
		f.Close()

		// Append Layer ID to Manifest File
		contentManifest.Layers = append(contentManifest.Layers, fakeLayerID+"/layer.tar")

		// Create JSON File for Layer (from emptyJson)
		if len(manifest.Layers)-1 == i { // Check if it's the last Layer
			createJSONLastLayerFile(parentID, fakeLayerID, config)
		} else {
			createJSONLayerFile(parentID, fakeLayerID, config)
		}
	}

	// Write Image Content Manifest to File
	contentData := make([]ContentManifest, 1)
	contentData[0] = contentManifest
	contentFile, _ := json.MarshalIndent(contentData, "", " ")
	_ = ioutil.WriteFile("./golayer/manifest.json", contentFile, 0644)

	// Create Repo File
	createRepoFile(imgFlag, *parentID)
	fmt.Println("\n \nFinished pulling " + imgFlag + ":" + tagFlag)
	// Create archive

	fmt.Println("Creating Tarball out of layers.....")
	tarErr := Tar("golayer", "image.tar")
	if tarErr != nil {
		fmt.Println(tarErr.Error())
	}
	// Cleanup
	os.RemoveAll("golayer/")
}

//checkValidInput tests if the input is not empty and contains the repository dash
func checkValidInput() bool {
	chk1 := imgFlag != "" && strings.Contains(imgFlag, "/")
	chk2 := tagFlag != ""
	return chk1 && chk2
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
	var jsonData LayerJSON
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
			Latest string `json:"latest"`
		}{digest},
	}
	repoFile, _ := json.MarshalIndent(repo, "", " ")
	_ = ioutil.WriteFile("./golayer/repositories", repoFile, 0644)
}

func downloadLayerBlob(image, blob, fakeLayerID string) error {
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
	if err != nil {
		return err
	}
	return nil
}

func generateFakeID(id, b string) string {
	s := id + "\n" + b + "\n"
	h := hmac.New(sha256.New, []byte("secret"))
	h.Write([]byte(s))

	return hex.EncodeToString(h.Sum(nil))
}

func downloadConfig(image, digest string) (ImageConfig, error) {
	uri := registryURL + "/v2/" + image + "/blobs/" + digest
	reqInfo, err := http.NewRequest("GET", uri, nil)
	reqInfo.Header.Add("Authorization", "Bearer "+authResult.Token)
	reqInfo.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")
	resp, err := api.Do(reqInfo)
	if err != nil {
		fmt.Println(err)
	}
	var result ImageConfig
	jsonErr := json.NewDecoder(resp.Body).Decode(&result)
	if jsonErr != nil {
		return ImageConfig{}, jsonErr
	}
	return result, nil
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
