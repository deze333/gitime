// Update GIT project files mtime to latest file commit.
// Usage is funky because git not always understands --work-tree.
// Need to cd to git repo dir first, then run this command:
// cd <git-proj-dir> && <this-exec-dir>/gitime
package main

import (
    "bytes"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path"
    "path/filepath"
    "strings"
    "time"
)

//------------------------------------------------------------
// Update GIT project files mtime to latest file commit
//------------------------------------------------------------

func main() {
    pwd, err := os.Getwd()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error getting working directory: %v", err)
        fmt.Println("Usage:")
        fmt.Println("cd <git-dir> && <bin-dir>/gitime")
        os.Exit(1)
    }

    logWalk(pwd)
}

// Get full commit list and files updated at each commit.
func logWalk(gitDir string) {
    // Get all commits
    hashes, err := getCommits()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error listing git commits: %v", err)
        os.Exit(1)
    }

    // For each commit
    for _, hash := range hashes {
        if hash == "" {
            continue
        }
        mtime, fs, err := getCommitFiles(hash)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error listing git commit files: %v", err)
            os.Exit(1)
        }

        // Update each file
        for _, f := range fs {
            fmt.Println(mtime, ":", f)

            fpath := path.Join(gitDir, f)
            if _, err := os.Stat(fpath); os.IsNotExist(err) {
                fmt.Fprintf(os.Stderr, "SKIP not existing file: %v", f)
                continue
            }

            // Change mtime of this file
            err = os.Chtimes(fpath, mtime, mtime)
            if err != nil {
                fmt.Fprintf(os.Stderr, "Error changing file mtime: %v", err)
                os.Exit(1)
            }
        }
    }
}

// Lists all commits.
func getCommits() (hashes []string, err error) {

    cmd := exec.Command("git", "log", "--pretty=%H")
    out, err := cmd.CombinedOutput()
    if err != nil {
        err = errors.New(string(out))
        return
    }

    hashes = strings.Split(string(out), "\n")
    return 
}

// Files changed in particular commit.
func getCommitFiles(hash string) (date time.Time, files []string, err error) {
    cmd := exec.Command("git", "show",  "--name-only", "--pretty=%ad", hash)
    out, err := cmd.CombinedOutput()
    if err != nil {
        err = errors.New(string(out))
        return
    }

    lines := strings.Split(string(out), "\n")

    date, err = time.Parse("Mon Jan 2 15:04:05 2006 -0700", lines[0])
    if err != nil {
        err = errors.New("Could not understand this time stamp: " + lines[0])
    }

    for i := 1; i < len(lines); i++ {
        if lines[i] == "" {
            continue
        }
        files = append(files, lines[i])
    }

    return
}

// Kept for historical purposes. Very slow.
func mainFilesWalk() {

    // Validate parameters
    if len(os.Args) != 2 {
        fmt.Fprintf(os.Stderr, "Provide git directory as argument")
        os.Exit(1)
    }

    // Validate GIT directory exists
    gitDir := path.Join(os.Args[1], ".git")
    if _, err := os.Stat(gitDir); os.IsNotExist(err) {
        fmt.Fprintf(os.Stderr, "GIT directory doesn't exist: %v", gitDir)
        os.Exit(1)
    }

    // Validate GIT work tree directory exists
    gitTreeDir := path.Join(os.Args[1])
    if _, err := os.Stat(gitTreeDir); os.IsNotExist(err) {
        fmt.Fprintf(os.Stderr, "GIT working tree directory doesn't exist: %v", gitTreeDir)
        os.Exit(1)
    }

    filesWalk(gitDir, gitTreeDir)
}

// Get file list then for each file find latest revision and mtime.
// Very Slow.
func filesWalk(gitDir, gitTreeDir string) {
    // Get list of project files
    fs, err := gitListFiles(gitDir)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error listing git files: %v", err)
        os.Exit(1)
    }

    // Process each file
    for _, f := range fs {

        // Omit empty
        if f == "" {
            continue
        }

        // Omit non existing
        fpath := path.Join(gitTreeDir, f)
        if _, err := os.Stat(fpath); os.IsNotExist(err) {
            continue
        }

        //fmt.Println(f)

        // Get latest revision
        rev, err := gitFileRevision(gitDir, gitTreeDir, f)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error reading file revision: %v", err)
            os.Exit(1)
        }

        //fmt.Println(rev)

        // Get commit date
        mtime, err := gitCommitTime(gitDir, gitTreeDir, rev)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error reading revision info: %v", err)
            os.Exit(1)
        }

        //fmt.Println(mtime)

        // Change mtime of this file
        err = os.Chtimes(fpath, mtime, mtime)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error changing file mtime: %v", err)
            os.Exit(1)
        }
    }
}

// Lists all files in GIT project.
func gitListFiles(gitDir string) (fs []string, err error) {

    cmd := exec.Command("git", "--git-dir=" + gitDir, "ls-files")
    out, err := cmd.CombinedOutput()
    if err != nil {
        err = errors.New(string(out))
        return
    }

    fs = strings.Split(string(out), "\n")
    return 
}

// Find last revision for given file.
func gitFileRevision(gitDir, gitTreeDir, f string) (rev string, err error) {

    //cmd := exec.Command("git", "--git-dir=" + gitDir, "--work-tree=" + gitTreeDir, "rev-list", "-n", "1", "HEAD", filepath.ToSlash(f))
    cmd := exec.Command("git", "rev-list", "-n", "1", "HEAD", filepath.ToSlash(f))
    out, err := cmd.CombinedOutput()
    if err != nil {
        err = errors.New(string(out))
        return
    }

    rev = strings.Trim(string(out), "\n")
    return
}

// Parses given time of commit.
func gitCommitTime(gitDir, gitTreeDir, hash string) (date time.Time, err error) {
    cmd := exec.Command("git", "show", "--pretty=format:%ai", hash)
    out, err := cmd.CombinedOutput()
    if err != nil {
        err = errors.New(string(out))
        return
    }

    idx := bytes.IndexAny(out, "\n")
    if idx == -1 {
        err = errors.New("Could not separate date from this output:\n" + string(out))
        return
    }

    date, err = time.Parse("2006-01-02 15:04:05 -0700", string(out[:idx]))
    if err != nil {
        err = errors.New("Could not understand this time stamp: " + string(out[:idx]))
    }
    return
}
