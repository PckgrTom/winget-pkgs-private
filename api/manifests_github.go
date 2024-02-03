package handler

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
)

var (
	WINGET_PKGS_OWNER     = os.Getenv("VERCEL_GIT_REPO_OWNER")
	WINGET_PKGS_REPO_NAME = os.Getenv("VERCEL_GIT_REPO_SLUG")
	WINGET_PKGS_BRANCH    = os.Getenv("VERCEL_GIT_COMMIT_REF")
)

func ManifestsGithub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pkg_id := r.URL.Query().Get("package_identifier")
	pkg_id = strings.ToLower(pkg_id)
	if pkg_id == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("package_identifier query parameter is required"))
		return
	}
	include_version_manifest := r.URL.Query().Get("include_version_manifest") == "true"

	res, err := http.Get(fmt.Sprintf("https://codeload.github.com/%s/%s/zip/refs/heads/%s", WINGET_PKGS_OWNER, WINGET_PKGS_REPO_NAME, WINGET_PKGS_BRANCH))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("error getting source zip: %s", err)))
		return
	}
	out, err := os.Create("/tmp/source.zip")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("error creating /tmp/source.zip file: %s", err)))
		return
	}
	defer out.Close()
	_, err = io.Copy(out, res.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("error writing to /tmp/source.zip file: %s", err)))
		return
	}

	srcZip, err := zip.OpenReader("/tmp/source.zip")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("error opening source.zip for reading: %s", err)))
		return
	}

	pkg_versions := getVersions(pkg_id, srcZip)
	if len(pkg_versions) == 0 {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("package %s not found", pkg_id)))
		return
	}

	result := []VersionAndManifests{}

	for _, version := range pkg_versions {
		manifests := getManifests(pkg_id, version, srcZip, include_version_manifest)
		result = append(result, VersionAndManifests{
			Version:   version,
			Manifests: manifests,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type VersionAndManifests struct {
	Version   string
	Manifests []Manifest
}

type Manifest struct {
	FileName string
	Content  string
}

func getVersions(pkg_id string, zipFile *zip.ReadCloser) []string {
	pkg_path := getPackagePath(pkg_id, "")
	versions := []string{}
	for _, file := range zipFile.File {
		file_name_lower := strings.ToLower(file.Name)
		if !strings.HasPrefix(file_name_lower, pkg_path) || !file.Mode().IsDir() {
			continue
		}
		version := file.Name[len(pkg_path)+1:]
		if slices.Contains(versions, version) || version == "" {
			continue
		}
		versions = append(versions, strings.TrimSuffix(version, "/"))
	}
	// remove sub-packages
	for i := 0; i < len(versions); i++ {
		is_sub_package := false
		for j := 0; j < len(versions); j++ {
			if strings.HasPrefix(versions[j], versions[i]) && versions[j] != versions[i] {
				is_sub_package = true
				versions = append(versions[:j], versions[j+1:]...)
				j--
			}
		}
		if is_sub_package {
			versions = append(versions[:i], versions[i+1:]...)
			i--
		}
	}
	return versions
}

func getManifests(pkg_id, version string, zipFile *zip.ReadCloser, include_version_manifest bool) []Manifest {
	pkg_path := getPackagePath(pkg_id, version)
	version_manfiest_path := getPackagePath(pkg_id, version, fmt.Sprintf("%s.yaml", pkg_id))
	manifests := []Manifest{}
	for _, file := range zipFile.File {
		file_name_lower := strings.ToLower(file.Name)
		if !strings.HasPrefix(file_name_lower, pkg_path) || !file.Mode().IsRegular() || (!include_version_manifest && file_name_lower == version_manfiest_path) {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return []Manifest{}
		}
		defer rc.Close()
		bytes, err := io.ReadAll(rc)
		if err != nil {
			return []Manifest{}
		}
		manifests = append(manifests, Manifest{
			FileName: strings.TrimPrefix(file.Name, fmt.Sprintf("%s-%s/", WINGET_PKGS_REPO_NAME, WINGET_PKGS_BRANCH)),
			Content:  string(bytes),
		})
	}
	return manifests
}

func getPackagePath(pkg_id, version string, fileName ...string) string {
	pkg_path := fmt.Sprintf("%s-%s/manifests/%s/%s", WINGET_PKGS_REPO_NAME, WINGET_PKGS_BRANCH, strings.ToLower(pkg_id[0:1]), strings.ReplaceAll(pkg_id, ".", "/"))
	if len(version) > 0 {
		pkg_path += "/" + version
	}
	if len(fileName) > 0 {
		pkg_path += "/" + fileName[0]
	}
	return pkg_path
}
