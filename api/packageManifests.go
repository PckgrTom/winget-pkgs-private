package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func PackageManifests(w http.ResponseWriter, r *http.Request) {
	// only allow GET requests
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pkg_id := r.URL.Query().Get("package_identifier")
	if pkg_id == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("package_identifier query parameter is required"))
		return
	}

	response := []ApiResponse{}
	res, err := http.Get("https://winget-pkgs-private.vercel.app/api/manifests_github?package_identifier=" + pkg_id)
	// error will only be of type *url.Error, so added check for status code as well
	if err != nil || res.StatusCode != http.StatusOK {
		// we assume that the error is because the package was not found because
		// the API seems to be stable 🙂 and the only error that can occur is when the package is not found
		w.WriteHeader(http.StatusNoContent)
		w.Write([]byte(fmt.Sprintf("package %s not found in repo", pkg_id)))
		return
	}
	defer res.Body.Close()
	json.NewDecoder(res.Body).Decode(&response)

	result_data_versions := []Result_Data_Versions{}
	for _, resp := range response {
		var installers, default_locale interface{}
		var locales []interface{}
		for _, manifest_raw := range resp.Manifests {
			manifest := yaml.MustParse(manifest_raw.Content + "\n---\n")

			manifest.PipeE(yaml.Clear("PackageIdentifier"))
			manifest.PipeE(yaml.Clear("PackageVersion"))
			manifest.PipeE(yaml.Clear("ManifestType"))
			manifest.PipeE(yaml.Clear("ManifestVersion"))

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
				yaml.Unmarshal([]byte(manifest.MustString()), &default_locale)
			case "locale":
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
			"PackageIdentifier": pkg_id,
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

type ApiResponse struct {
	Version   string `json:"Version"`
	Manifests []struct {
		FileName string `json:"FileName"`
		Content  string `json:"Content"`
	} `json:"Manifests"`
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
