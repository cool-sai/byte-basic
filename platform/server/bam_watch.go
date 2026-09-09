package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func (s *server) watchBamGit() {
	s.pollBamGit()
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for range t.C {
		s.pollBamGit()
	}
}

func (s *server) pollBamGit() {
	rows, err := s.db.Query(`SELECT DISTINCT scm_name FROM bam_module WHERE scm_name!=''`)
	if err != nil {
		log.Println("bam watch", err)
		return
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			log.Println("bam watch", err)
			return
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		log.Println("bam watch", err)
		return
	}
	for _, scm := range names {
		if err := s.pollScm(scm); err != nil {
			log.Println("bam watch", scm, err)
		}
	}
}

func (s *server) pollScm(scm string) error {
	job, err := s.getJob(scm)
	if err != nil {
		return err
	}
	branches, err := gitLsRemote(job.GitURL)
	if err != nil {
		return err
	}
	for _, b := range branches {
		if b.Name == "" || b.Commit == "" {
			continue
		}
		var old string
		err := s.db.QueryRow(`SELECT git_commit FROM bam_git_ref WHERE scm_name=? AND branch=?`, scm, b.Name).Scan(&old)
		if err == sql.ErrNoRows {
			if _, err := s.db.Exec(
				`INSERT INTO bam_git_ref (scm_name, branch, git_commit) VALUES (?,?,?)`,
				scm, b.Name, b.Commit,
			); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if old == b.Commit {
			continue
		}
		if err := s.genScmBranch(scm, b.Name, b.Commit); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) genScmBranch(scm, branch, commit string) error {
	rows, err := s.db.Query(`SELECT name FROM bam_module WHERE scm_name=?`, scm)
	if err != nil {
		return err
	}
	defer rows.Close()
	var mods []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return err
		}
		mods = append(mods, n)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(mods) == 0 {
		return nil
	}
	ok := 0
	for _, name := range mods {
		if _, err := s.syncAndGen(name, branch); err != nil {
			log.Println("bam watch gen", name, branch, err)
			continue
		}
		ok++
	}
	if ok == 0 {
		return fmt.Errorf("bam gen failed %s %s", scm, branch)
	}
	_, err = s.db.Exec(
		`INSERT INTO bam_git_ref (scm_name, branch, git_commit) VALUES (?,?,?)
		 ON DUPLICATE KEY UPDATE git_commit=VALUES(git_commit)`,
		scm, branch, commit,
	)
	return err
}

func (s *server) handleBamHook(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	branch, commit, repoURL := parseGitHook(raw)
	if branch == "" || commit == "" || strings.Trim(commit, "0") == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	want := normalizeGitURL(repoURL)
	rows, err := s.db.Query(`SELECT name, repo_dir FROM scm_job`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var scms []string
	for rows.Next() {
		var name, url string
		if err := rows.Scan(&name, &url); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if normalizeGitURL(url) == want {
			scms = append(scms, name)
		}
	}
	for _, scm := range scms {
		if err := s.genScmBranch(scm, branch, commit); err != nil {
			log.Println("bam hook", scm, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseGitHook(raw []byte) (branch, commit, repoURL string) {
	var gh struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Deleted    bool   `json:"deleted"`
		Repository struct {
			CloneURL     string `json:"clone_url"`
			GitHTTPURL   string `json:"git_http_url"`
			SSHURL       string `json:"ssh_url"`
		} `json:"repository"`
		Project struct {
			GitHTTPURL string `json:"git_http_url"`
			HTTPURL    string `json:"http_url"`
		} `json:"project"`
	}
	if err := json.Unmarshal(raw, &gh); err != nil {
		return "", "", ""
	}
	if gh.Deleted {
		return "", "", ""
	}
	branch = strings.TrimPrefix(gh.Ref, "refs/heads/")
	if branch == gh.Ref {
		return "", "", ""
	}
	commit = gh.After
	repoURL = gh.Repository.CloneURL
	if repoURL == "" {
		repoURL = gh.Repository.GitHTTPURL
	}
	if repoURL == "" {
		repoURL = gh.Project.GitHTTPURL
	}
	if repoURL == "" {
		repoURL = gh.Project.HTTPURL
	}
	if repoURL == "" {
		repoURL = gh.Repository.SSHURL
	}
	return branch, commit, repoURL
}
