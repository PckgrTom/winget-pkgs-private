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

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

var (
	WINGET_PKGS_OWNER     = os.Getenv("VERCEL_GIT_REPO_OWNER")
	WINGET_PKGS_REPO_NAME = os.Getenv("VERCEL_GIT_REPO_SLUG")
	WINGET_PKGS_BRANCH    = os.Getenv("VERCEL_GIT_COMMIT_REF")
)

func PackageManifests(w http.ResponseWriter, r *http.Request) {
	// only allow GET requests
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if os.Getenv("AUTH") != "" && r.Header.Get("Windows-Package-Manager") != os.Getenv("AUTH") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	pkg_id := r.URL.Query().Get("package_identifier")
	if pkg_id == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("package_identifier query parameter is required"))
		return
	}

	url := fmt.Sprintf("https://codeload.github.com/%s/%s/zip/refs/heads/%s", WINGET_PKGS_OWNER, WINGET_PKGS_REPO_NAME, WINGET_PKGS_BRANCH)
	path := "/tmp/winget-pkgs.zip"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := downloadRepository(url, path)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error downloading winget-pkgs repository: %s", err)
			return
		}
	}

	repoZip, err := zip.OpenReader(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error opening %s for reading: %s", path, err)
		return
	}

	pkg_versions, pkg_id_proper := getVersions(strings.ToLower(pkg_id), repoZip)
	if len(pkg_versions) == 0 {
		w.WriteHeader(http.StatusNoContent)
		w.Write([]byte(fmt.Sprintf("package %s not found in repo", pkg_id)))
		return
	}

	result := []VersionAndManifests{}
	for _, version := range pkg_versions {
		manifests := getManifests(strings.ToLower(pkg_id), version, repoZip)
		result = append(result, VersionAndManifests{
			Version:   version,
			Manifests: manifests,
		})
	}

	result_data_versions := []Result_Data_Versions{}
	for _, resp := range result {
		var installers, default_locale interface{}
		var locales []interface{}
		for _, manifest_raw := range resp.Manifests {
			manifest := yaml.MustParse(manifest_raw.Content + "\n---\n")
			switch manifest.Field("ManifestType").Value.YNode().Value {
			case "installer":
				// copy from root level to installer level
				_installers, _ := manifest.GetSlice("Installers")
				no_of_installers := len(_installers)
				for _, field := range common_fields_root_and_installer {
					root_level_value, _ := manifest.Pipe(yaml.Get(field))
					if root_level_value == nil {
						continue
					}
					for i := 0; i < no_of_installers; i++ {
						if installer_level_value, _ := manifest.Pipe(yaml.Lookup("Installers"),
							yaml.Lookup(fmt.Sprintf("%d", i)),
							yaml.Get(field)); installer_level_value == nil {
							manifest.PipeE(yaml.Lookup("Installers"),
								yaml.Lookup(fmt.Sprintf("%d", i)),
								yaml.SetField(field, root_level_value))
						} else if installer_level_value.YNode().Kind == yaml.MappingNode {
							root_level_value_map, installer_level_keys := map[string]string{}, []string{}
							root_level_value.VisitFields(func(node *yaml.MapNode) error {
								root_level_value_map[node.Key.YNode().Value] = node.Value.YNode().Value
								return nil
							})
							installer_level_value.VisitFields(func(node *yaml.MapNode) error {
								installer_level_keys = append(installer_level_keys, node.Key.YNode().Value)
								return nil
							})
							for root_level_value_map_key, root_level_value_map_value := range root_level_value_map {
								if !slices.Contains(installer_level_keys, root_level_value_map_key) {
									manifest.PipeE(yaml.Lookup("Installers"),
										yaml.Lookup(fmt.Sprintf("%d", i)),
										yaml.Lookup(field),
										yaml.SetField(root_level_value_map_key, yaml.NewStringRNode(root_level_value_map_value)))
								}
							}
						}

					}
					manifest.PipeE(yaml.Clear(field))
				}

				installers_new, _ := manifest.Pipe(yaml.Get("Installers"))
				yaml.Unmarshal([]byte(installers_new.MustString()), &installers)
			case "defaultLocale":
				manifest.PipeE(yaml.Clear("PackageIdentifier"))
				manifest.PipeE(yaml.Clear("PackageVersion"))
				manifest.PipeE(yaml.Clear("ManifestType"))
				manifest.PipeE(yaml.Clear("ManifestVersion"))

				yaml.Unmarshal([]byte(manifest.MustString()), &default_locale)
			case "locale":
				manifest.PipeE(yaml.Clear("PackageIdentifier"))
				manifest.PipeE(yaml.Clear("PackageVersion"))
				manifest.PipeE(yaml.Clear("ManifestType"))
				manifest.PipeE(yaml.Clear("ManifestVersion"))

				var locale interface{}
				yaml.Unmarshal([]byte(manifest.MustString()), &locale)
				locales = append(locales, locale)
			}
		}
		result_data_versions = append(result_data_versions, Result_Data_Versions{
			PackageVersion: resp.Version,
			DefaultLocale:  default_locale,
			Installers:     installers,
			Locales:        locales,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]map[string]interface{}{
		"Data": {
			"PackageIdentifier": pkg_id_proper,
			"Versions":          result_data_versions,
		},
	})
}

type Result_Data_Versions struct {
	PackageVersion string
	DefaultLocale  interface{}
	Installers     interface{}
	Locales        []interface{}
}

type VersionAndManifests struct {
	Version   string
	Manifests []Manifest
}

type Manifest struct {
	FileName string
	Content  string
}

func getVersions(pkg_id string, zipFile *zip.ReadCloser) ([]string, string) {
	var pkg_id_proper string
	var versions []string
	pkg_path := getPackagePath(pkg_id, "")
	for _, file := range zipFile.File {
		file_name_lower := strings.ToLower(file.Name)
		if !strings.HasPrefix(file_name_lower, pkg_path) || !file.Mode().IsDir() {
			continue
		}
		if v := file.Name[len(pkg_path)+1:]; v != "" {
			versions = append(versions, strings.TrimSuffix(v, "/"))
		} else {
			// length of WINGET_PKGS_REPO + "-" + WINGET_PKGS_BRANCH + "/manifests/" + "m" + "/"
			pkg_id_proper = strings.ReplaceAll(file.Name[len(WINGET_PKGS_BRANCH)+len(WINGET_PKGS_REPO_NAME)+14:len(file.Name)-len(version)-1], "/", ".")
		}
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
	return versions, pkg_id_proper
}

func getManifests(pkg_id, version string, zipFile *zip.ReadCloser) []Manifest {
	pkg_path := getPackagePath(pkg_id, version)
	version_manfiest_path := getPackagePath(pkg_id, version, fmt.Sprintf("%s.yaml", pkg_id))
	manifests := []Manifest{}
	for _, file := range zipFile.File {
		file_name_lower := strings.ToLower(file.Name)
		if !strings.HasPrefix(file_name_lower, pkg_path) || !file.Mode().IsRegular() || file_name_lower == version_manfiest_path {
			continue
		}
		rc, _ := file.Open()
		bytes, _ := io.ReadAll(rc)
		manifests = append(manifests, Manifest{
			FileName: strings.TrimPrefix(file.Name, fmt.Sprintf("%s-%s/", WINGET_PKGS_REPO_NAME, WINGET_PKGS_BRANCH)),
			Content:  string(bytes),
		})
	}
	return manifests
}

func downloadRepository(url, path string) error {
	res, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error getting response: %s", err)
	}
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error creating %s file: %s", path, err)
	}
	_, err = io.Copy(out, res.Body)
	if err != nil {
		return fmt.Errorf("error writing to %s file: %s", path, err)
	}
	return nil
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

var common_fields_root_and_installer = []string{
	"InstallerLocale",
	"Platform",
	"MinimumOSVersion",
	"InstallerType",
	"NestedInstallerType",
	"NestedInstallerFiles",
	"Scope",
	"InstallModes",
	"InstallerSwitches",
	"InstallerSuccessCodes",
	"ExpectedReturnCodes",
	"UpgradeBehavior",
	"Commands",
	"Protocols",
	"FileExtensions",
	"Dependencies",
	"PackageFamilyName",
	"ProductCode",
	"Capabilities",
	"RestrictedCapabilities",
	"Markets",
	"InstallerAbortsTerminal",
	"ReleaseDate",
	"InstallLocationRequired",
	"RequireExplicitUpgrade",
	"DisplayInstallWarnings",
	"UnsupportedOSArchitectures",
	"UnsupportedArguments",
	"AppsAndFeaturesEntries",
	"ElevationRequirement",
	"InstallationMetadata"}
