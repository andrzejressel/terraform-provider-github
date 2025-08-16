package github

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/google/go-github/v66/github"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceGithubIssueComment() *schema.Resource {
	return &schema.Resource{
		Create: resourceGithubIssueCommentCreateOrUpdate,
		Read:   resourceGithubIssueCommentRead,
		Update: resourceGithubIssueCommentCreateOrUpdate,
		Delete: resourceGithubIssueCommentDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"repository": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The GitHub repository.",
			},
			"issue_number": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "The number that identifies the issue.",
			},
			"body": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The contents of the comment.",
			},
			"comment_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The comment id.",
			},
		},
	}
}

// Create or update an issue comment. During creation we only know issue_number; on update we derive comment ID from state.
func resourceGithubIssueCommentCreateOrUpdate(d *schema.ResourceData, meta interface{}) error {
	ctx := context.Background()
	client := meta.(*Owner).v3client
	orgName := meta.(*Owner).name
	repoName := d.Get("repository").(string)
	body := d.Get("body").(string)
	req := &github.IssueComment{Body: github.String(body)}

	var comment *github.IssueComment
	var resp *github.Response
	var err error

	if d.IsNewResource() {
		issueNumber := d.Get("issue_number").(int)
		comment, resp, err = client.Issues.CreateComment(ctx, orgName, repoName, issueNumber, req)
		if resp != nil {
			log.Printf("[DEBUG] Response from creating issue: %#v", *resp)
		}
	} else {
		commentID := d.Get("comment_id").(int64)
		_, resp, err = client.Issues.EditComment(ctx, orgName, repoName, commentID, req)
		if resp != nil {
			log.Printf("[DEBUG] Response from updating issue: %#v", *resp)
		}
	}
	if err != nil {
		return err
	}

	d.SetId(buildTwoPartID(repoName, strconv.FormatInt(comment.GetID(), 10)))
	if err = d.Set("comment_id", comment.GetID()); err != nil {
		return err
	}
	return resourceGithubIssueCommentRead(d, meta)
}

func resourceGithubIssueCommentRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Owner).v3client
	repoName, commentIDStr, err := parseTwoPartID(d.Id(), "repository", "comment_id")
	if err != nil {
		return err
	}
	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil {
		return unconvertibleIdErr(commentIDStr, err)
	}

	orgName := meta.(*Owner).name
	ctx := context.WithValue(context.Background(), ctxId, d.Id())

	githubComment, _, err := client.Issues.GetComment(ctx, orgName, repoName, commentID)
	if err != nil {
		return err
	}

	if err = d.Set("repository", repoName); err != nil {
		return err
	}
	if err = d.Set("comment_id", githubComment.GetID()); err != nil {
		return err
	}
	if err = d.Set("body", githubComment.GetBody()); err != nil {
		return err
	}

	issue_url := githubComment.GetIssueURL()
	parts := strings.Split(issue_url, "/")
	if len(parts) > 0 {
		issue_id, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			return nil
		}
		if err = d.Set("issue_number", issue_id); err != nil {
			return err
		}
	}

	return nil
}

func resourceGithubIssueCommentDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Owner).v3client

	orgName := meta.(*Owner).name
	repoName := d.Get("repository").(string)
	commentID := d.Get("comment_id").(int64)

	ctx := context.WithValue(context.Background(), ctxId, d.Id())
	_, err := client.Issues.DeleteComment(ctx, orgName, repoName, commentID)
	return err
}
