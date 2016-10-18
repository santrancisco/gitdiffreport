package githubwraper

import (
	"log"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
)

type reportstruct struct {
	Name             string
	Language         string
	Size             int
	IsTested         bool
	Fork             bool
	Private          bool
	OverCompareLimit bool
	CloneURL         string
	NumberOfRelease  int
	*checkpointstruct
	CreateAt         time.Time
	PushAt           time.Time
	LastCommitSHA    string
	TotalCommitsDiff int
	Changes          map[string]*changestruct
	Messages         []string
}

//status: 0 - added , 1 - modified , 2 - deleted
type changestruct struct {
	Additions int
	Deletions int
}

type checkpointstruct struct {
	Repo           string
	CheckpointSHA  string
	CheckpointTime time.Time
}

// Gitstruct which contains the client to github, the list of secops members,
// the list of repositories and the latest sha1 commit we checked
type Gitstruct struct {
	IgnoreRepos     []string
	IgnoreExtension []string
	Client          *github.Client
	Org             string
	Token           string
	Secopsmembers   []string
	Repositories    map[string]*checkpointstruct
	Reports         map[string]*reportstruct
}

// NewGitstruct is constructor for Gitstruct
func NewGitstruct(org, githubToken string) (newstruct Gitstruct, err error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	newstruct = Gitstruct{Client: github.NewClient(tc),
		Org: org, Token: githubToken,
		Repositories: map[string]*checkpointstruct{},
		Reports:      map[string]*reportstruct{}}

	err = newstruct.GetSecopMembers()
	if err != nil {
		return Gitstruct{}, err
	}
	err = newstruct.GetRepositories()
	if err != nil {
		return Gitstruct{}, err
	}
	return newstruct, err
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func safeAssignString(spointer *string) string {
	if spointer == nil {
		return "None"
	}
	return *spointer
}

func getExtension(filename string) (ext string) {
	filename = getLast(strings.Split(filename, "/"))
	ext = "no_extension"
	if strings.Contains(filename, ".") {
		ext = getLast(strings.Split(filename, "."))
	}
	return ext
}

//Helper function to get last element of string array
func getLast(stringarray []string) string {
	return stringarray[len(stringarray)-1]
}

//GetSecopMembers get the list of Secops members
func (g *Gitstruct) GetSecopMembers() error {
	log.Println("Getting Secops members")
	teams, _, err := g.Client.Organizations.ListTeams(g.Org, &github.ListOptions{PerPage: 100})
	if err != nil {
		return (err)
	}
	for _, t := range teams {
		if *t.Name == "Security" {
			log.Println("Getting members of Security team")
			users, _, err := g.Client.Organizations.ListTeamMembers(*t.ID, nil)
			if err != nil {
				return err
			}
			for _, u := range users {
				if !contains(g.Secopsmembers, *u.Login) {
					g.Secopsmembers = append(g.Secopsmembers, *u.Login)
				}
			}
			log.Printf("New list of secops member: %s\n", g.Secopsmembers)
		}
	}
	return nil
}

//GetRepositories get the list of repositories for the organization
func (g *Gitstruct) GetRepositories() error {
	log.Println("Getting list of Repositories")
	repositories, _, err := g.Client.Repositories.ListByOrg(g.Org, &github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: 300}})
	if err != nil {
		return (err)
	}
	for _, r := range repositories {
		// If this is a new repository, add it to the list. We also only care about repositories changed in the last 1 month for now
		if _, exist := g.Repositories[*r.Name]; !exist && (time.Now().AddDate(0, -1, 0)).Before(r.PushedAt.Time) {
			log.Printf("%s - %s", *r.Name, r.PushedAt.Time)
			// Initialise CheckpointSHA with "new" indicating pentest has not happened.
			g.Repositories[*r.Name] = &checkpointstruct{Repo: *r.Name, CheckpointSHA: "new"}
			err = g.GetRepoStat(*r.Name)
			if err != nil {
				return (err)
			}
		}
	}
	//log.Printf("New list of repository :\n %s\n", g.Repositories)
	return nil
}

//GetPentestCheckpoint get the latest pentest checkpoint
func (g *Gitstruct) GetPentestCheckpoint(repo string) error {
	log.Printf("Getting pentest checkpoint for %s", repo)
	issues, _, err := g.Client.Issues.ListByRepo(g.Org, repo, &github.IssueListByRepoOptions{State: "all", Labels: []string{"question"}})
	if err != nil {
		return (err)
	}
	var latest time.Time
	for _, i := range issues {
		// If the pentest checkpoint issue is raised by one of the pentester and is latest, update it
		if (latest.Before(*i.CreatedAt)) && (contains(g.Secopsmembers, *i.User.Login)) {
			latest = *i.CreatedAt
			log.Printf("Update latest with: %s - %s created issue %s, state(%s) with description:\n%s", *i.CreatedAt, *i.User.Login, *i.Title, *i.State, *i.Body)
			g.Repositories[repo].Repo = repo
			g.Repositories[repo].CheckpointTime = *i.CreatedAt
			g.Repositories[repo].CheckpointSHA = strings.TrimSpace(*i.Body)
			g.Reports[repo].IsTested = true
		}
	}
	return nil
}

//GetRepoStat fetch the stat of a repository for report
func (g *Gitstruct) GetRepoStat(repo string) error {
	log.Printf("Getting stat for %s", repo)
	repostat, _, err := g.Client.Repositories.Get(g.Org, repo)
	if err != nil {
		return (err)
	}

	if _, exist := g.Reports[repo]; !exist {
		// Create new if not exist
		g.Reports[repo] = &reportstruct{}
		g.Reports[repo].Changes = map[string]*changestruct{}
	}
	g.Reports[repo].Name = repo
	g.Reports[repo].Language = safeAssignString(repostat.Language)
	g.Reports[repo].Size = *repostat.Size
	g.Reports[repo].CreateAt = repostat.CreatedAt.Time
	g.Reports[repo].PushAt = repostat.PushedAt.Time
	g.Reports[repo].CloneURL = safeAssignString(repostat.CloneURL)
	g.Reports[repo].NumberOfRelease = 0
	g.Reports[repo].Fork = *repostat.Fork
	g.Reports[repo].Private = *repostat.Private
	g.Reports[repo].OverCompareLimit = false
	g.Reports[repo].TotalCommitsDiff = -1
	lastcommit, _, err := g.Client.Repositories.ListCommits(g.Org, repo, &github.CommitsListOptions{ListOptions: github.ListOptions{PerPage: 1}})
	if err != nil {
		return (err)
	}
	// Update the pentest checkpoint for this repository
	g.Reports[repo].IsTested = false
	err = g.GetPentestCheckpoint(repo)
	if err != nil {
		return (err)
	}
	g.Reports[repo].checkpointstruct = g.Repositories[repo]

	// Note: this check may not be neccessary as we will re-run every morning and init new object anyway.
	if g.Reports[repo].LastCommitSHA != *lastcommit[0].SHA {
		// We have some new commits!
		log.Println("We got some new commits!")
		g.Reports[repo].LastCommitSHA = *lastcommit[0].SHA
		err = g.GetCommitDiff(repo, g.Reports[repo].checkpointstruct.CheckpointSHA, g.Reports[repo].LastCommitSHA)
		err = g.GetReleaseTags(repo)
		if err != nil {
			return (err)
		}
	}
	return nil
}

// GetCommitMessages gets commit messages for the report.
func (g *Gitstruct) GetCommitMessages(repo string, gitdiff *github.CommitsComparison) error {
	for _, commit := range gitdiff.Commits {
		message := *commit.Commit.Message
		wholecommit, _, err := g.Client.Repositories.GetCommit(g.Org, repo, *commit.SHA)
		if err != nil {
			return (err)
		}
		interested := false
		for _, file := range wholecommit.Files {
			if !contains(g.IgnoreExtension, getExtension(*file.Filename)) {
				// If any file is not in the list of ignore-extension, we will probably be interested in the commit message
				interested = true
				break
			}
		}
		if interested {
			//log.Println("Interested in : " + message)
			g.Reports[repo].Messages = append(g.Reports[repo].Messages, message)
		}
	}
	return nil
}

// GetReleaseTags get all the release tag for this project
func (g *Gitstruct) GetReleaseTags(repo string) error {
	tags, _, err := g.Client.Repositories.ListTags(g.Org, repo, nil)
	if err != nil {
		return err
	}
	if g.Reports[repo].CheckpointSHA == "new" {
		g.Reports[repo].NumberOfRelease = len(tags)
		return nil
	}
	// Check which release tag was created since our last checkpoint pentest
	counter := 0
	for _, tag := range tags {
		diff, _, err := g.Client.Repositories.CompareCommits(g.Org, repo, *tag.Commit.SHA, g.Reports[repo].LastCommitSHA)
		if err != nil {
			continue
		}
		if *diff.TotalCommits > g.Reports[repo].TotalCommitsDiff {
			break
		}
		counter++
	}
	g.Reports[repo].NumberOfRelease = counter
	return nil
}

//GetCommitDiff behaves like gitdiff and gives us a report of what has changed since the last pentest checkpoint
func (g *Gitstruct) GetCommitDiff(repo, sha1before, sha1after string) error {
	if g.Reports[repo].IsTested {
		log.Printf("Got a repo been tested here! %s\n", repo)
		diff, _, err := g.Client.Repositories.CompareCommits(g.Org, repo, sha1before, sha1after)
		if err != nil {
			return (err)
		}
		g.Reports[repo].TotalCommitsDiff = *diff.TotalCommits
		// TODO: git api compare can only handle up to 250 commits at a time
		// Do we need to go through every single commits if that is the case?
		if *diff.TotalCommits > 250 {
			g.Reports[repo].OverCompareLimit = true
		}
		for _, file := range diff.Files {
			extension := getExtension(*file.Filename)
			if _, exist := g.Reports[repo].Changes[extension]; !exist {
				g.Reports[repo].Changes[extension] = &changestruct{Additions: 0, Deletions: 0}
			}
			g.Reports[repo].Changes[extension].Additions += *file.Additions
			g.Reports[repo].Changes[extension].Deletions += *file.Deletions

		}
		err = g.GetCommitMessages(repo, diff)
		if err != nil {
			return (err)
		}

	} else {
		//If this is a brand new project that has not been tested previously.
		log.Printf("Got a new repo here! Never been touched %s\n", repo)

	}
	return nil
}
