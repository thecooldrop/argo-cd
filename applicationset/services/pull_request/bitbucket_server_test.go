package pull_request

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func defaultHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100":
			_, err = io.WriteString(w, `{
					"size": 1,
					"limit": 100,
					"isLastPage": true,
					"values": [
						{
							"id": 101,
							"toRef": {
								"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
								"displayId": "master",
								"id": "refs/heads/master"
							},
							"fromRef": {
								"id": "refs/heads/feature-ABC-123",
								"displayId": "feature-ABC-123",
								"latestCommit": "cb3cf2e4d1517c83e720d2585b9402dbef71f992"
							}
						}
					],
					"start": 0
				}`)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}
}

func TestListPullRequestNoAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO")
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, 101, pullRequests[0].Number)
	assert.Equal(t, "feature-ABC-123", pullRequests[0].Branch)
	assert.Equal(t, "master", pullRequests[0].TargetBranch)
	assert.Equal(t, "cb3cf2e4d1517c83e720d2585b9402dbef71f992", pullRequests[0].HeadSHA)
}

func TestListPullRequestPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100":
			_, err = io.WriteString(w, `{
					"size": 2,
					"limit": 2,
					"isLastPage": false,
					"values": [
						{
							"id": 101,
							"toRef": {
								"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
								"displayId": "master",
								"id": "refs/heads/master"
							},
							"fromRef": {
								"id": "refs/heads/feature-101",
								"displayId": "feature-101",
								"latestCommit": "ab3cf2e4d1517c83e720d2585b9402dbef71f992"
							}
						},
						{
							"id": 102,
							"toRef": {
								"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
								"displayId": "branch",
								"id": "refs/heads/branch"
							},
							"fromRef": {
								"id": "refs/heads/feature-102",
								"displayId": "feature-102",
								"latestCommit": "bb3cf2e4d1517c83e720d2585b9402dbef71f992"
							}
						}
					],
					"nextPageStart": 200
				}`)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100&start=200":
			_, err = io.WriteString(w, `{
				"size": 1,
				"limit": 2,
				"isLastPage": true,
				"values": [
					{
						"id": 200,
						"toRef": {
							"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
							"displayId": "master",
							"id": "refs/heads/master"
						},
						"fromRef": {
							"id": "refs/heads/feature-200",
							"displayId": "feature-200",
							"latestCommit": "cb3cf2e4d1517c83e720d2585b9402dbef71f992"
						}
					}
				],
				"start": 200
			}`)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}))
	defer ts.Close()
	svc, err := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO")
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 3)
	assert.Equal(t, PullRequest{
		Number:       101,
		Branch:       "feature-101",
		TargetBranch: "master",
		HeadSHA:      "ab3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
	}, *pullRequests[0])
	assert.Equal(t, PullRequest{
		Number:       102,
		Branch:       "feature-102",
		TargetBranch: "branch",
		HeadSHA:      "bb3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
	}, *pullRequests[1])
	assert.Equal(t, PullRequest{
		Number:       200,
		Branch:       "feature-200",
		TargetBranch: "master",
		HeadSHA:      "cb3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
	}, *pullRequests[2])
}

func TestListPullRequestBasicAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// base64(user:password)
		assert.Equal(t, "Basic dXNlcjpwYXNzd29yZA==", r.Header.Get("Authorization"))
		assert.Equal(t, "no-check", r.Header.Get("X-Atlassian-Token"))
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketServiceBasicAuth(context.Background(), "user", "password", ts.URL, "PROJECT", "REPO")
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, 101, pullRequests[0].Number)
	assert.Equal(t, "feature-ABC-123", pullRequests[0].Branch)
	assert.Equal(t, "cb3cf2e4d1517c83e720d2585b9402dbef71f992", pullRequests[0].HeadSHA)
}

func TestListPullRequestBearerAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tolkien", r.Header.Get("Authorization"))
		assert.Equal(t, "no-check", r.Header.Get("X-Atlassian-Token"))
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketServiceBearerToken(context.Background(), "tolkien", ts.URL, "PROJECT", "REPO")
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, 101, pullRequests[0].Number)
	assert.Equal(t, "feature-ABC-123", pullRequests[0].Branch)
	assert.Equal(t, "cb3cf2e4d1517c83e720d2585b9402dbef71f992", pullRequests[0].HeadSHA)
}

func TestListResponseError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	svc, _ := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO")
	_, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.Error(t, err)
}

func TestListResponseMalformed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100":
			_, err := io.WriteString(w, `{
					"size": 1,
					"limit": 100,
					"isLastPage": true,
					"values": { "id": 101 },
					"start": 0
				}`)
			if err != nil {
				t.Fail()
			}
		default:
			t.Fail()
		}
	}))
	defer ts.Close()
	svc, _ := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO")
	_, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.Error(t, err)
}

func TestListResponseEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100":
			_, err := io.WriteString(w, `{
					"size": 0,
					"limit": 100,
					"isLastPage": true,
					"values": [],
					"start": 0
				}`)
			if err != nil {
				t.Fail()
			}
		default:
			t.Fail()
		}
	}))
	defer ts.Close()
	svc, err := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO")
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Empty(t, pullRequests)
}

func TestListPullRequestBranchMatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100":
			_, err = io.WriteString(w, `{
					"size": 2,
					"limit": 2,
					"isLastPage": false,
					"values": [
						{
							"id": 101,
							"toRef": {
								"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
								"displayId": "master",
								"id": "refs/heads/master"
							},
							"fromRef": {
								"id": "refs/heads/feature-101",
								"displayId": "feature-101",
								"latestCommit": "ab3cf2e4d1517c83e720d2585b9402dbef71f992"
							}
						},
						{
							"id": 102,
							"toRef": {
								"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
								"displayId": "branch",
								"id": "refs/heads/branch"
							},
							"fromRef": {
								"id": "refs/heads/feature-102",
								"displayId": "feature-102",
								"latestCommit": "bb3cf2e4d1517c83e720d2585b9402dbef71f992"
							}
						}
					],
					"nextPageStart": 200
				}`)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100&start=200":
			_, err = io.WriteString(w, `{
				"size": 1,
				"limit": 2,
				"isLastPage": true,
				"values": [
					{
						"id": 200,
						"toRef": {
							"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
							"displayId": "master",
							"id": "refs/heads/master"
						},
						"fromRef": {
							"id": "refs/heads/feature-200",
							"displayId": "feature-200",
							"latestCommit": "cb3cf2e4d1517c83e720d2585b9402dbef71f992"
						}
					}
				],
				"start": 200
			}`)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}))
	defer ts.Close()
	regexp := `feature-1[\d]{2}`
	svc, err := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO")
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: &regexp,
		},
	})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 2)
	assert.Equal(t, PullRequest{
		Number:       101,
		Branch:       "feature-101",
		TargetBranch: "master",
		HeadSHA:      "ab3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
	}, *pullRequests[0])
	assert.Equal(t, PullRequest{
		Number:       102,
		Branch:       "feature-102",
		TargetBranch: "branch",
		HeadSHA:      "bb3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
	}, *pullRequests[1])

	regexp = `.*2$`
	svc, err = NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO")
	require.NoError(t, err)
	pullRequests, err = ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: &regexp,
		},
	})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, PullRequest{
		Number:       102,
		Branch:       "feature-102",
		TargetBranch: "branch",
		HeadSHA:      "bb3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
	}, *pullRequests[0])

	regexp = `[\d{2}`
	svc, err = NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO")
	require.NoError(t, err)
	_, err = ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: &regexp,
		},
	})
	require.Error(t, err)
}
