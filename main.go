// This is a Github Action (https://developer.github.com/actions/) that attempts
// to auto-tag releases.
//
// This action is meant to be triggered by a 'pull_request' change and therefore
// receives from Github a PullRequestEvent from which to infer the information
// needed to work its magic. It only increments the revision. For major and
// minor changes, we can manually set a new tag.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/google/go-github/v29/github"
	"github.com/hashicorp/go-version"
	"golang.org/x/oauth2"
)

var (
	exConfig  = 78 // special code github actions take as "no error, but stop processing after that"
	fatalExit = 1  // error code to exit with on fatal/fatalf
)

func usage() {
	fmt.Println("Usage: autotagger")
	fmt.Println("You can also set the following environment variables:")
	fmt.Println("    NO_EX_CONFIG     disables the EX_CONFIG returns, returning success instead")
	fmt.Println("    NEVER_FAIL       in cases where the bot should fail, it will return EX_CONFIG instead")
	fmt.Println("    FILE_REGEXP      only tag when changes since the last tag include files that match this regex (default: .*).")
	fmt.Println("    TAG_PREFIX       prefix your tag with this. Great for Go modules in a subdir!")

	os.Exit(fatalExit)
}

func main() {
	if os.Getenv("NO_EX_CONFIG") == "true" {
		exConfig = 0
	}

	// aka the John Wick mode
	if os.Getenv("NEVER_FAIL") == "true" {
		fatalExit = exConfig
	}

	fileRE := ".*"
	if fe, ok := os.LookupEnv("FILE_REGEXP"); ok {
		fileRE = fe
	}

	fileMatch := regexp.MustCompile(fileRE)

	prefix := os.Getenv("TAG_PREFIX")

	// limit this action to pull requests only
	triggerName := os.Getenv("GITHUB_EVENT_NAME")
	if triggerName != "pull_request" {
		log.Printf("Ignoring trigger %s", triggerName)
		os.Exit(exConfig)
	}

	// create a github client
	tok := os.Getenv("GITHUB_TOKEN")
	if tok == "" {
		fatal("You must enable GITHUB_TOKEN access for this action")
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
	oc := oauth2.NewClient(context.Background(), ts)
	c := github.NewClient(oc)

	// Read the trigger event information
	b, err := ioutil.ReadFile(os.Getenv("GITHUB_EVENT_PATH"))
	if err != nil {
		fatalf("could not read event info: %v", err)
	}

	var se github.PullRequestEvent
	if err := json.Unmarshal(b, &se); err != nil {
		fatalf("could not unmarshal event info: %v", err)
	}

	if *se.Action != "closed" || !*se.PullRequest.Merged {
		fmt.Printf("PR not ready to tag (action: %s, merged: %v)\n", *se.Action, *se.PullRequest.Merged)
		os.Exit(exConfig)
	}

	ref := se.PullRequest.GetMergeCommitSHA()
	if ref == "" {
		fatal("Could not find the merge commit")
	}

	ctx := context.Background()

	owner, repo := se.GetRepo().GetOwner().GetLogin(), se.GetRepo().GetName()
	cli := &client{c, owner, repo}

	lastVersion, err := cli.getLastVersion(ctx, prefix)
	if err != nil {
		fatal(err)
	}

	base := prefix + "v" + lastVersion.String()

	if !cli.shouldTag(ctx, base, ref, fileMatch) {
		fmt.Println("No changes matching pattern. This code won't be tagged.")
		return
	}

	version := nextVersion(lastVersion, prefix)

	_, _, err = c.Git.CreateRef(ctx, owner, repo, &github.Reference{
		Ref:    github.String(fmt.Sprintf("refs/tags/%s", version)),
		Object: &github.GitObject{SHA: &ref, Type: github.String("commit")},
	})
	if err != nil {
		fatalf("could not create tag for ref %s: %v", ref, err)
	}

	fmt.Println("Tagged version", version)

	_, _, err = c.Issues.CreateComment(ctx, owner, repo, se.PullRequest.GetNumber(), &github.IssueComment{
		Body: github.String(fmt.Sprintf("Your friendly autotagging bot has tagged this as release **%s**", version)),
	})
	if err != nil {
		fatalf("could not create comment: %v", err)
	}
	fmt.Println("Done")
}

type client struct {
	c     *github.Client
	owner string
	repo  string
}

func (c *client) getLastVersion(ctx context.Context, prefix string) (*version.Version, error) {
	last, err := version.NewSemver("v0.0.0")
	if err != nil {
		return nil, fmt.Errorf("could not create base version: %v", err)
	}

	page := 1
	for {
		lo := &github.ReferenceListOptions{
			Type: "tag",
			ListOptions: github.ListOptions{
				Page: page,
			},
		}
		refs, resp, err := c.c.Git.ListRefs(ctx, c.owner, c.repo, lo)
		if err != nil {
			return nil, err
		}

		for _, r := range refs {
			fmt.Println("Ref:", r.GetRef())

			tag := strings.TrimPrefix(r.GetRef(), "refs/tags/"+prefix)
			v, err := version.NewSemver(tag)
			if err != nil {
				fmt.Printf("Tag %v is not a valid semver, ignoring", tag)
				continue
			}
			if v.GreaterThan(last) {
				fmt.Println("Found newer version:", v)
				last = v
			}
		}

		// do we have more?
		link := resp.Header.Get("Link")
		if strings.Index(link, "rel=\"next\"") == -1 {
			// we're done here
			break
		}
		page++
	}

	if last.String() == "0.0.0" {
		return nil, errors.New("could not find any versions")
	}

	return last, nil
}

func (c *client) shouldTag(ctx context.Context, base, merge string, fileMatch *regexp.Regexp) bool {

	// repositories service compare commits
	cmp, _, err := c.c.Repositories.CompareCommits(ctx, c.owner, c.repo, base, merge)
	if err != nil {
		fatal("error getting diff:", err)
	}

	for _, cf := range cmp.Files {
		if fileMatch.MatchString(*cf.Filename) {
			return true
		}
	}

	return false
}

func nextVersion(v *version.Version, prefix string) string {
	segs := v.Segments()
	diff := 3 - len(segs)
	for i := 0; i < diff; i++ {
		segs = append(segs, 0)
	}

	return fmt.Sprintf("%sv%d.%d.%d", prefix, segs[0], segs[1], segs[2]+1)
}

// fatal is like log.Fatal but respects NEVER_FAIL
func fatal(a ...interface{}) {
	log.Print(a...)
	os.Exit(fatalExit)
}

// fatal is like log.Fatalf but respects NEVER_FAIL
func fatalf(frmt string, a ...interface{}) {
	fatal(fmt.Sprintf(frmt, a...))
}
