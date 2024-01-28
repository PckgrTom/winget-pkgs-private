package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/google/go-github/v58/github"
	"github.com/maruel/natural"
	"golang.org/x/oauth2"
)

type Manifest struct {
	FileName string
	Content  string
}

func ManifestsGithub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	var github_client *github.Client
	var WINGET_PKGS_OWNER string

	if val, ok := os.LookupEnv("GITHUB_PAT"); !ok {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("GITHUB_PAT environment variable is not set."))
		return
	} else {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: val},
		)
		tc := oauth2.NewClient(ctx, ts)
		github_client = github.NewClient(tc)
		if user, _, err := github_client.Users.Get(context.Background(), ""); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Invalid GitHub token"))
			return
		} else {
			WINGET_PKGS_OWNER = user.GetLogin() // get winget-pkgs owner from token
		}
	}
	
	const WINGET_PKGS_REPO_NAME = "winget-pkgs-private"

	pkg_id := r.URL.Query().Get("package_identifier")
	if (pkg_id == "") {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("package_identifier query parameter is required"))
		return
	}

	_, versions_in_dir, _, err := github_client.Repositories.GetContents(context.Background(), WINGET_PKGS_OWNER, WINGET_PKGS_REPO_NAME, getPackagePath(pkg_id, ""), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("error getting package versions for %s: %s", pkg_id, err)))
		return
	}

	pkg_versions := []string{}
	commonly_ignored_versions := []string{"eap", "preview", "beta", "dev", "nightly", "canary", "insiders"}
	for _, dir_content := range versions_in_dir {
		if dir_content.GetType() == "dir" && !slices.Contains(commonly_ignored_versions, strings.ToLower(dir_content.GetName())) {
			pkg_versions = append(pkg_versions, dir_content.GetName())
		}
	}

	// sort and get latest version
	sort.Sort(natural.StringSlice(pkg_versions))
	version := pkg_versions[len(pkg_versions)-1]

	_, dir_contents, _, err := github_client.Repositories.GetContents(context.Background(), WINGET_PKGS_OWNER, WINGET_PKGS_REPO_NAME, getPackagePath(pkg_id, version), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("error getting manifests for %s version %s: %s", pkg_id, version, err)))
		return
	}

	manifests := []Manifest{}
	for _, dir_content := range dir_contents {
		res, err := http.Get(dir_content.GetDownloadURL())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("error getting manifest %s: %s", dir_content.GetName(), err)))
			return
		}
		manifest_raw, err := io.ReadAll(res.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("error reading response body for manifest %s: %s", dir_content.GetName(), err)))
			return
		}
		defer res.Body.Close()
		manifests = append(manifests, Manifest{
			FileName: getPackagePath(pkg_id, version, dir_content.GetName()),
			Content:  string(manifest_raw),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(manifests)
}

func getPackagePath(pkg_id, version string, fileName ...string) string {
	pkg_path := fmt.Sprintf("manifests/%s/%s", strings.ToLower(pkg_id[0:1]), strings.ReplaceAll(pkg_id, ".", "/"))
	if len(version) > 0 {
		pkg_path += "/" + version
	}
	if len(fileName) > 0 {
		pkg_path += "/" + fileName[0]
	}
	return pkg_path
}
