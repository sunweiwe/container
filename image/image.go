//Package image for docker
package image

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sunweiwe/container/common"
	"github.com/sunweiwe/container/tar"
	"github.com/sunweiwe/container/utils"
)

type imageEntries map[string]string
type imagesCache map[string]imageEntries

func DownloadImageIfRequired(src string) string {
	imageName, tag := getImageNameAndTag(src)
	if downloadRequired, imageHash := ImageExistByTag(imageName, tag); !downloadRequired {
		/* Setup the imageExistByTag we want to pull */
		log.Printf("Downloading metadata for %s:%s, please wait...", imageName, tag)
		img, err := crane.Pull(strings.Join([]string{imageName, tag}, ":"))
		if err != nil {
			log.Fatal(err)
		}

		manifest, _ := img.Manifest()
		imageHash = manifest.Config.Digest.Hex[:12]
		log.Printf("imageHash: %v\n", imageHash)
		log.Println("Checking if image exists under another name...")
		/* Identify cases where ubuntu:latest could be the same as ubuntu:20.04*/

		alterImageName, alterImageTag := imageExistByHash(imageHash)
		if len(alterImageName) > 0 && len(alterImageTag) > 0 {
			log.Printf("The image you request %s:%s is the same as %s:%s\n", imageName, tag, alterImageName, alterImageTag)
			storeImageMetadata(imageName, tag, imageHash)
			return imageHash
		} else {
			log.Printf("Image doesn't exist. Downloading...")
			downloadImage(img, imageHash, src)
			untarFile(imageHash)
			processLayerTarballs(imageHash, manifest.Config.Digest.Hex)
			// 保存image信息
			storeImageMetadata(imageName, tag, imageHash)
			clearTemporaryImageFile(imageHash)
			return imageHash
		}

	} else {
		log.Println("Image already exists. Not downloading.")
		return imageHash
	}

}

func getImageNameAndTag(imageName string) (string, string) {
	s := strings.Split(imageName, ":")
	var img, tag string
	if len(s) > 1 {
		img, tag = s[0], s[1]
	} else {
		img = s[0]
		tag = "latest"
	}

	return img, tag
}

// imageExistByHash
func imageExistByHash(imageHash string) (string, string) {
	imgCache := imagesCache{}
	parseImagesMetadata(&imgCache)
	for imageName, alterImages := range imgCache {
		for imageTag, imgHash := range alterImages {
			if imageHash == imgHash {
				return imageName, imageTag
			}
		}
	}
	return "", ""
}

func ImageExistByTag(imageName string, tagName string) (bool, string) {
	imgCaches := imagesCache{}
	parseImagesMetadata(&imgCaches)
	for k, v := range imgCaches {
		if k == imageName {
			for k, v := range v {
				if k == tagName {
					return true, v
				}
			}
		}
	}

	return false, ""
}

func parseImagesMetadata(imgCache *imagesCache) {
	// TODO to constant or config
	imagesCachePath := "/var/lib/container/images/images.json"

	// check file exist or not
	if _, err := os.Stat(imagesCachePath); os.IsNotExist(err) {
		/* If it doesn't exist create an empty Cache json */
		os.WriteFile(imagesCachePath, []byte("{}"), 0644)
	}

	data, err := os.ReadFile(imagesCachePath)

	if err != nil {
		log.Fatalf("Could not read images Cache: %v\n", err)
	}

	if err := json.Unmarshal(data, imgCache); err != nil {
		log.Fatalf("Unable to parse images Cache: %v\n", err)
	}
}

func storeImageMetadata(image string, tag string, imageHash string) {
	imageCaches := imagesCache{}
	imageEntry := imageEntries{}
	parseImagesMetadata(&imageCaches)
	if imageCaches[image] != nil {
		imageEntry = imageCaches[image]
	}
	imageEntry[tag] = imageHash
	imageCaches[image] = imageEntry

	marshaImageMetadata(imageCaches)
}

func marshaImageMetadata(imageCaches imagesCache) {
	fileBytes, err := json.Marshal(imageCaches)
	if err != nil {
		log.Printf("Unable to marshall images data:%v\n", err)
	}

	// TODO
	imagesCachePath := "/var/lib/container/images/images.json"
	if err := os.WriteFile(imagesCachePath, fileBytes, 0644); err != nil {
		log.Printf("Unable to save images Caches:%v \n", err)
	}
}

func downloadImage(image v1.Image, imageHash string, src string) {
	path := "/var/lib/container/tmp/" + imageHash
	os.Mkdir(path, 0755)
	path += "/package.tar"
	// save the image as a tar file
	if err := crane.SaveLegacy(image, src, path); err != nil {
		log.Printf("saving tarball %s: %v", path, err)
	}
	log.Printf("Successfully downloaded %s\n", imageHash)

}

func untarFile(imageHash string) {
	log.Printf("untarFile %s\n", imageHash)

	pathDir := "/var/lib/container/tmp/" + imageHash
	pathTar := pathDir + "/package.tar"
	if err := tar.Untar(pathTar, pathDir, false); err != nil {
		log.Printf("Error untaring file: %v\n", err)
	}
}

//processLayerTarballs
func processLayerTarballs(imageHash string, fullImageHex string) {
	tPathDir := "/var/lib/container/tmp/" + imageHash
	pathManifest := tPathDir + "/manifest.json"
	pathConfig := tPathDir + "/" + "sha256:" + fullImageHex
	fmt.Printf("processLayerTarballs pathConfig is %s ", pathConfig)
	mani := common.Manifest{}
	//解析 manifest文件，拿到当前的image结构
	utils.ParseManifest(pathManifest, &mani)
	if len(mani) == 0 || len(mani[0].Layers) == 0 {
		log.Fatal("Could not find any layer.")
	}
	if len(mani) > 1 {
		log.Fatal("I don't know how to handle more than one manifest.")
	}

	imageDir := "/var/lib/container/images/" + imageHash
	_ = os.Mkdir(imageDir, 0755)
	// untar the layer files. These become the basic of our container root fs
	for _, layer := range mani[0].Layers {
		imageLayerDir := imageDir + "/" + layer[:12] + "/fs"
		log.Printf("Uncompressed layer to: %s \n", imageLayerDir)
		// 创建文件夹
		_ = os.MkdirAll(imageLayerDir, 0755)
		srcLayer := tPathDir + "/" + layer
		if err := tar.Untar(srcLayer, imageLayerDir, true); err != nil {
			log.Fatalf("Unable to untar layer file: %s \n:%v", srcLayer, err)
		}
	}

	// 复制 manifest 文件
	utils.CopyFile(pathManifest, "/var/lib/container/images/"+imageHash+"/"+imageHash+".json")
	utils.CopyFile(pathConfig, "/var/lib/container/images/"+imageHash+"/"+imageHash)
}

func clearTemporaryImageFile(imageHash string) {
	tPath := "/var/lib/container/temp/" + imageHash
	utils.DoOrDieWithMessage(os.RemoveAll(tPath), "Unable to remove temporary image files")
}

func ParseContainerConfig(imageHash string) common.ImageConfig {
	imagesConfigPath := "/var/lib/container/images/" + imageHash + "/" + imageHash
	data, err := os.ReadFile(imagesConfigPath)
	if err != nil {
		log.Fatalf("Could not read image config file")
	}
	imgConfig := common.ImageConfig{}
	if err := json.Unmarshal(data, &imgConfig); err != nil {
		log.Fatalf("Unable to parse image config data!")
	}

	return imgConfig
}

func GetImageAndTagForHash(imageHash string) (string, string) {
	imgCache := imagesCache{}
	parseImagesMetadata(&imgCache)
	for image, versions := range imgCache {
		for version, hash := range versions {
			if hash == imageHash {
				return image, version
			}
		}
	}

	return "", ""
}

func RemoveImageMetadata(imageHash string) {
	imgCache := imagesCache{}
	imageEntry := imageEntries{}
	parseImagesMetadata(&imgCache)
	imageName, _ := imageExistByHash(imageHash)
	if len(imageName) == 0 {
		log.Fatalf("Could not get image details")
	}
	imageEntry = imgCache[imageName]
	for tag, hash := range imageEntry {
		if hash == imageHash {
			delete(imageEntry, tag)
		}
	}

	if len(imageEntry) == 0 {
		delete(imgCache, imageName)
	} else {
		imgCache[imageName] = imageEntry
	}
	marshaImageMetadata(imgCache)
}

func PrintAvailableImages() {
	imgCache := imagesCache{}
	parseImagesMetadata(&imgCache)
	fmt.Printf("IMAGE\t     TAG\t ID\n")
	for image, detail := range imgCache {
		for tag, hash := range detail {
			fmt.Printf("%s\t %10s\t %s\n", image, tag, hash)
		}
	}
}
