package send

import (
	"fmt"
	"log"
	"os"

	"github.com/google/go-github/github"
	"github.com/mongodb/grip/message"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

type githubLogger struct {
	opts GithubOptions
	gh   githubClient

	*Base
}

// GithubOptions contains information about a github account and
// repository, used in the GithubIssuesLogger and the
// GithubCommentLogger Sender implementations.
type GithubOptions struct {
	Account string
	Repo    string
	Token   string
}

// NewGitHubIssuesLogger builds a sender implementation that creates a
// new issue in a Github Project for each log message.
func NewGithubIssuesLogger(name string, opts GithubOptions) (Sender, error) {
	s := &githubLogger{
		Base: NewBase(name),
		opts: opts,
		gh:   &githubClientImpl{},
	}

	ctx := context.TODO()
	s.gh.Init(ctx, opts.Token)

	fallback := log.New(os.Stdout, "", log.LstdFlags)
	if err := s.SetErrorHandler(ErrorHandlerFromLogger(fallback)); err != nil {
		return nil, err
	}

	if err := s.SetFormatter(MakeDefaultFormatter()); err != nil {
		return nil, err
	}

	s.reset = func() {
		fallback.SetPrefix(fmt.Sprintf("[%s] [%s/%s] ", s.Name(), opts.Account, opts.Repo))
	}

	return s, nil
}

func (s *githubLogger) Send(m message.Composer) {
	if s.level.ShouldLog(m) {
		text, err := s.formatter(m)
		if err != nil {
			s.errHandler(err, m)
			return
		}

		title := fmt.Sprintf("[%s]: %s", s.Name(), m.String())
		issue := &github.IssueRequest{
			Title: &title,
			Body:  &text,
		}

		ctx := context.TODO()
		if _, _, err := s.gh.Create(ctx, s.opts.Account, s.opts.Repo, issue); err != nil {
			s.errHandler(err, m)
		}
	}
}

//////////////////////////////////////////////////////////////////////////
//
// interface wrapper for the github client so that we can mock things out
//
//////////////////////////////////////////////////////////////////////////

type githubClient interface {
	Init(context.Context, string)
	Create(context.Context, string, string, *github.IssueRequest) (*github.Issue, *github.Response, error)
	CreateComment(context.Context, string, string, int, *github.IssueComment) (*github.IssueComment, *github.Response, error)
}

type githubClientImpl struct {
	*github.IssuesService
}

func (c *githubClientImpl) Init(ctx context.Context, token string) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})

	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	c.IssuesService = client.Issues
}
