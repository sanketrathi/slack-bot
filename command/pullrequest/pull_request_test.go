package pullrequest

import (
	"github.com/innogames/slack-bot/bot"
	"github.com/innogames/slack-bot/bot/config"
	"github.com/innogames/slack-bot/bot/matcher"
	"github.com/innogames/slack-bot/client"
	"github.com/innogames/slack-bot/mocks"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type testFetcher struct {
	pr  pullRequest
	err error
}

func (t testFetcher) getPullRequest(match matcher.Result) (pullRequest, error) {
	return t.pr, t.err
}

func (t testFetcher) getHelp() []bot.Help {
	return []bot.Help{}
}

func TestGetCommands(t *testing.T) {
	slackClient := &mocks.SlackClient{}
	cfg := config.Config{}
	logger := logrus.New()

	// as we pass a empty config, no PR fetcher is able to register -> 0 valid commands
	commands := GetCommands(slackClient, cfg, logger)
	assert.Equal(t, 0, commands.Count())
}

func TestPullRequest(t *testing.T) {
	slackClient := &mocks.SlackClient{}

	t.Run("invalid command", func(t *testing.T) {
		commands, _ := initTest(slackClient)

		event := slack.MessageEvent{}
		event.Text = "quatsch"

		actual := commands.Run(event)
		assert.Equal(t, false, actual)
	})

	t.Run("PR not found", func(t *testing.T) {
		commands, fetcher := initTest(slackClient)

		event := slack.MessageEvent{}
		fetcher.err = errors.New("PR not found")
		event.Text = "vcd.example.com/projects/foo/repos/bar/pull-requests/1337"

		slackClient.On("ReplyError", event, fetcher.err)

		actual := commands.Run(event)
		assert.Equal(t, true, actual)
	})

	t.Run("PR got merged", func(t *testing.T) {
		commands, fetcher := initTest(slackClient)

		event := slack.MessageEvent{}
		fetcher.err = nil
		fetcher.pr = pullRequest{
			declined:  false,
			merged:    true,
			approvers: []string{"test"},
			inReview:  false,
		}
		event.Text = "vcd.example.com/projects/foo/repos/bar/pull-requests/1337"

		msgRef := slack.NewRefToMessage(event.Channel, event.Timestamp)
		slackClient.
			On("GetReactions", msgRef, slack.NewGetReactionsParameters()).Return(nil, nil)

		slackClient.On("RemoveReaction", iconInReview, slack.NewRefToMessage(event.Channel, event.Timestamp))
		slackClient.On("AddReaction", iconApproved, slack.NewRefToMessage(event.Channel, event.Timestamp))
		slackClient.On("AddReaction", iconMerged, slack.NewRefToMessage(event.Channel, event.Timestamp))

		actual := commands.Run(event)
		assert.Equal(t, true, actual)
		time.Sleep(time.Millisecond * 10) // todo channel
	})

	t.Run("PR got declined", func(t *testing.T) {
		commands, fetcher := initTest(slackClient)

		event := slack.MessageEvent{}
		fetcher.err = nil
		fetcher.pr = pullRequest{
			declined:  true,
			merged:    false,
			approvers: []string{},
			inReview:  false,
		}
		event.Text = "vcd.example.com/projects/foo/repos/bar/pull-requests/1337"

		slackClient.On("RemoveReaction", iconInReview, slack.NewRefToMessage(event.Channel, event.Timestamp))
		slackClient.On("RemoveReaction", iconApproved, slack.NewRefToMessage(event.Channel, event.Timestamp))
		slackClient.On("AddReaction", iconDeclined, slack.NewRefToMessage(event.Channel, event.Timestamp))

		actual := commands.Run(event)
		assert.Equal(t, true, actual)
		time.Sleep(time.Millisecond * 10) // todo channel
	})

	t.Run("PR got approvers", func(t *testing.T) {
		commands, fetcher := initTest(slackClient)

		event := slack.MessageEvent{}
		fetcher.err = nil
		fetcher.pr = pullRequest{
			declined:  false,
			merged:    false,
			approvers: []string{"test"},
			inReview:  false,
		}
		event.Text = "vcd.example.com/projects/foo/repos/bar/pull-requests/1337"

		slackClient.On("RemoveReaction", iconInReview, slack.NewRefToMessage(event.Channel, event.Timestamp))
		slackClient.On("RemoveReaction", iconDeclined, slack.NewRefToMessage(event.Channel, event.Timestamp))
		slackClient.On("AddReaction", iconApproved, slack.NewRefToMessage(event.Channel, event.Timestamp))

		actual := commands.Run(event)
		assert.Equal(t, true, actual)
		time.Sleep(time.Millisecond * 10) // todo channel
	})

	t.Run("PR in reiew", func(t *testing.T) {
		commands, fetcher := initTest(slackClient)

		event := slack.MessageEvent{}
		fetcher.err = nil
		fetcher.pr = pullRequest{
			declined:  false,
			merged:    false,
			approvers: []string{},
			inReview:  true,
		}
		event.Text = "vcd.example.com/projects/foo/repos/bar/pull-requests/1337"

		slackClient.On("AddReaction", iconInReview, slack.NewRefToMessage(event.Channel, event.Timestamp))

		actual := commands.Run(event)
		assert.Equal(t, true, actual)
		time.Sleep(time.Millisecond * 10) // todo channel
	})
}

// creates a fresh instance of Commands a clean Fetcher to avoid racing conditions
func initTest(slackClient client.SlackClient) (bot.Commands, *testFetcher) {
	fetcher := &testFetcher{}
	commands := bot.Commands{}
	logger := logrus.New()

	cmd := &command{
		config.PullRequest{},
		slackClient,
		logger,
		fetcher,
		".*/projects/(?P<project>.+)/repos/(?P<repo>.+)/pull-requests/(?P<number>\\d+).*",
	}
	commands.AddCommand(cmd)

	return commands, fetcher
}
