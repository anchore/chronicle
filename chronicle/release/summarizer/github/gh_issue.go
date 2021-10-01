package github

import (
	"context"
	"os"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

type ghIssue struct {
	Title    string
	Number   int
	Author   string
	ClosedAt time.Time
	Labels   []string
	URL      string
}

type issueFilter func(issue ghIssue) bool

func filterIssues(issues []ghIssue, filters ...issueFilter) []ghIssue {
	if len(filters) == 0 {
		return issues
	}

	results := make([]ghIssue, 0, len(issues))

issueLoop:
	for _, r := range issues {
		for _, f := range filters {
			if !f(r) {
				continue issueLoop
			}
		}
		results = append(results, r)
	}

	return results
}

func issuesAfter(since time.Time) issueFilter {
	return func(issue ghIssue) bool {
		return issue.ClosedAt.After(since)
	}
}

func issuesBefore(since time.Time) issueFilter {
	return func(issue ghIssue) bool {
		return issue.ClosedAt.Before(since)
	}
}

func issuesWithLabel(labels ...string) issueFilter {
	return func(issue ghIssue) bool {
		for _, l := range issue.Labels {
			for _, targetLabel := range labels {
				if l == targetLabel {
					return true
				}
			}
		}
		return false
	}
}

func fetchClosedIssues(user, repo string) ([]ghIssue, error) {
	src := oauth2.StaticTokenSource(
		// TODO: DI this
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)
	var allIssues []ghIssue

	// Query some details about a repository, an ghIssue in it, and its comments.
	{
		// TODO: act on hitting a rate limit
		type rateLimit struct {
			Cost      githubv4.Int
			Limit     githubv4.Int
			Remaining githubv4.Int
			ResetAt   githubv4.DateTime
		}

		var query struct {
			Repository struct {
				DatabaseID githubv4.Int
				URL        githubv4.URI
				Issues     struct {
					PageInfo struct {
						EndCursor   githubv4.String
						HasNextPage bool
					}
					Edges []struct {
						Node struct {
							Title  githubv4.String
							Number githubv4.Int
							URL    githubv4.String
							Author struct {
								Login githubv4.String
							}
							ClosedAt githubv4.DateTime
							Labels   struct {
								Edges []struct {
									Node struct {
										Name githubv4.String
									}
								}
							} `graphql:"labels(first:100)"`
						}
					}
				} `graphql:"issues(first:100, states:CLOSED, after:$issuesCursor)"`
			} `graphql:"repository(owner:$repositoryOwner, name:$repositoryName)"`

			RateLimit rateLimit
		}
		variables := map[string]interface{}{
			"repositoryOwner": githubv4.String(user),
			"repositoryName":  githubv4.String(repo),
			"issuesCursor":    (*githubv4.String)(nil), // Null after argument to get first page.
		}

		//var limit rateLimit
		for {
			err := client.Query(context.Background(), &query, variables)
			if err != nil {
				return nil, err
			}
			//limit = query.RateLimit

			for _, iEdge := range query.Repository.Issues.Edges {
				var labels []string
				for _, lEdge := range iEdge.Node.Labels.Edges {
					labels = append(labels, string(lEdge.Node.Name))
				}
				allIssues = append(allIssues, ghIssue{
					Title:    string(iEdge.Node.Title),
					Author:   string(iEdge.Node.Author.Login),
					ClosedAt: iEdge.Node.ClosedAt.Time,
					Labels:   labels,
					URL:      string(iEdge.Node.URL),
					Number:   int(iEdge.Node.Number),
				})
			}

			if !query.Repository.Issues.PageInfo.HasNextPage {
				break
			}
			variables["issuesCursor"] = githubv4.NewString(query.Repository.Issues.PageInfo.EndCursor)
		}

		//for idx, is := range allIssues {
		//	fmt.Printf("%d: %+v\n", idx, is)
		//}
		//printJSON(limit)
	}

	return allIssues, nil
}

//func closedGithubIssuesByLabel(user, repo, label string) ([]ghIssue, error) {
//	src := oauth2.StaticTokenSource(
//		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
//	)
//	httpClient := oauth2.NewClient(context.Background(), src)
//	client := githubv4.NewClient(httpClient)
//	var allIssues []ghIssue
//
//	// Query some details about a repository, an ghIssue in it, and its comments.
//	{
//
//		// TODO: act on rate limit hits
//		type rateLimit struct {
//			Cost      githubv4.Int
//			Limit     githubv4.Int
//			Remaining githubv4.Int
//			ResetAt   githubv4.DateTime
//		}
//
//		var query struct {
//			Repository struct {
//				DatabaseID githubv4.Int
//				URL        githubv4.URI
//				Label      struct {
//					Name   githubv4.String
//					Issues struct {
//						PageInfo struct {
//							EndCursor   githubv4.String
//							HasNextPage bool
//						}
//						Edges []struct {
//							Node struct {
//								Title  githubv4.String
//								Author struct {
//									Login githubv4.String
//								}
//								// TODO: add assignees
//								ClosedAt githubv4.DateTime
//								Labels   struct {
//									Edges []struct {
//										Node struct {
//											Name githubv4.String
//										}
//									}
//								} `graphql:"labels(first:100)"`
//							}
//						}
//					} `graphql:"issues(first:100, states:CLOSED, after:$issuesCursor)"`
//				} `graphql:"label(name:$labelName)"`
//			} `graphql:"repository(owner:$repositoryOwner, name:$repositoryName)"`
//
//			RateLimit rateLimit
//		}
//		variables := map[string]interface{}{
//			"repositoryOwner": githubv4.String(user),
//			"repositoryName":  githubv4.String(repo),
//			"labelName":       githubv4.String(label),
//			"issuesCursor":    (*githubv4.String)(nil), // Null after argument to get first page.
//		}
//
//		//var limit rateLimit
//		for {
//			err := client.Query(context.Background(), &query, variables)
//			if err != nil {
//				return nil, err
//			}
//			//limit = query.RateLimit
//
//			for _, iEdge := range query.Repository.Label.Issues.Edges {
//				var labels []string
//				for _, lEdge := range iEdge.Node.Labels.Edges {
//					labels = append(labels, string(lEdge.Node.Name))
//				}
//				allIssues = append(allIssues, ghIssue{
//					Title:    string(iEdge.Node.Title),
//					Author:   string(iEdge.Node.Author.Login),
//					ClosedAt: iEdge.Node.ClosedAt.Time,
//					Labels:   labels,
//				})
//			}
//
//			if !query.Repository.Label.Issues.PageInfo.HasNextPage {
//				break
//			}
//			variables["issuesCursor"] = githubv4.NewString(query.Repository.Label.Issues.PageInfo.EndCursor)
//		}
//
//		//for idx, is := range allIssues {
//		//	fmt.Printf("%d: %+v\n", idx, is)
//		//}
//		//fmt.Println("Label: ", query.Repository.Label.Name)
//		//printJSON(limit)
//	}
//
//	return allIssues, nil
//}

//// printJSON prints v as JSON encoded with indent to stdout. It panics on any error.
//func printJSON(v interface{}) {
//	w := json.NewEncoder(os.Stdout)
//	w.SetIndent("", "\t")
//	err := w.Encode(v)
//	if err != nil {
//		panic(err)
//	}
//}
