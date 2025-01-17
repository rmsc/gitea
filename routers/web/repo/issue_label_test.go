// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"strconv"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"

	"github.com/stretchr/testify/assert"
)

func int64SliceToCommaSeparated(a []int64) string {
	s := ""
	for i, n := range a {
		if i > 0 {
			s += ","
		}
		s += strconv.Itoa(int(n))
	}
	return s
}

func TestInitializeLabels(t *testing.T) {
	db.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/labels/initialize")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 2)
	web.SetForm(ctx, &forms.InitializeLabelsForm{TemplateName: "Default"})
	InitializeLabels(ctx)
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	db.AssertExistsAndLoadBean(t, &models.Label{
		RepoID: 2,
		Name:   "enhancement",
		Color:  "#84b6eb",
	})
	assert.Equal(t, "/user2/repo2/labels", test.RedirectURL(ctx.Resp))
}

func TestRetrieveLabels(t *testing.T) {
	db.PrepareTestEnv(t)
	for _, testCase := range []struct {
		RepoID           int64
		Sort             string
		ExpectedLabelIDs []int64
	}{
		{1, "", []int64{1, 2}},
		{1, "leastissues", []int64{2, 1}},
		{2, "", []int64{}},
	} {
		ctx := test.MockContext(t, "user/repo/issues")
		test.LoadUser(t, ctx, 2)
		test.LoadRepo(t, ctx, testCase.RepoID)
		ctx.Req.Form.Set("sort", testCase.Sort)
		RetrieveLabels(ctx)
		assert.False(t, ctx.Written())
		labels, ok := ctx.Data["Labels"].([]*models.Label)
		assert.True(t, ok)
		if assert.Len(t, labels, len(testCase.ExpectedLabelIDs)) {
			for i, label := range labels {
				assert.EqualValues(t, testCase.ExpectedLabelIDs[i], label.ID)
			}
		}
	}
}

func TestNewLabel(t *testing.T) {
	db.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/labels/edit")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	web.SetForm(ctx, &forms.CreateLabelForm{
		Title: "newlabel",
		Color: "#abcdef",
	})
	NewLabel(ctx)
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	db.AssertExistsAndLoadBean(t, &models.Label{
		Name:  "newlabel",
		Color: "#abcdef",
	})
	assert.Equal(t, "/user2/repo1/labels", test.RedirectURL(ctx.Resp))
}

func TestUpdateLabel(t *testing.T) {
	db.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/labels/edit")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	web.SetForm(ctx, &forms.CreateLabelForm{
		ID:    2,
		Title: "newnameforlabel",
		Color: "#abcdef",
	})
	UpdateLabel(ctx)
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	db.AssertExistsAndLoadBean(t, &models.Label{
		ID:    2,
		Name:  "newnameforlabel",
		Color: "#abcdef",
	})
	assert.Equal(t, "/user2/repo1/labels", test.RedirectURL(ctx.Resp))
}

func TestDeleteLabel(t *testing.T) {
	db.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/labels/delete")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	ctx.Req.Form.Set("id", "2")
	DeleteLabel(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	db.AssertNotExistsBean(t, &models.Label{ID: 2})
	db.AssertNotExistsBean(t, &models.IssueLabel{LabelID: 2})
	assert.Equal(t, ctx.Tr("repo.issues.label_deletion_success"), ctx.Flash.SuccessMsg)
}

func TestUpdateIssueLabel_Clear(t *testing.T) {
	db.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/issues/labels")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	ctx.Req.Form.Set("issue_ids", "1,3")
	ctx.Req.Form.Set("action", "clear")
	UpdateIssueLabel(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	db.AssertNotExistsBean(t, &models.IssueLabel{IssueID: 1})
	db.AssertNotExistsBean(t, &models.IssueLabel{IssueID: 3})
	models.CheckConsistencyFor(t, &models.Label{})
}

func TestUpdateIssueLabel_Toggle(t *testing.T) {
	for _, testCase := range []struct {
		Action      string
		IssueIDs    []int64
		LabelID     int64
		ExpectedAdd bool // whether we expect the label to be added to the issues
	}{
		{"attach", []int64{1, 3}, 1, true},
		{"detach", []int64{1, 3}, 1, false},
		{"toggle", []int64{1, 3}, 1, false},
		{"toggle", []int64{1, 2}, 2, true},
	} {
		db.PrepareTestEnv(t)
		ctx := test.MockContext(t, "user2/repo1/issues/labels")
		test.LoadUser(t, ctx, 2)
		test.LoadRepo(t, ctx, 1)
		ctx.Req.Form.Set("issue_ids", int64SliceToCommaSeparated(testCase.IssueIDs))
		ctx.Req.Form.Set("action", testCase.Action)
		ctx.Req.Form.Set("id", strconv.Itoa(int(testCase.LabelID)))
		UpdateIssueLabel(ctx)
		assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
		for _, issueID := range testCase.IssueIDs {
			db.AssertExistsIf(t, testCase.ExpectedAdd, &models.IssueLabel{
				IssueID: issueID,
				LabelID: testCase.LabelID,
			})
		}
		models.CheckConsistencyFor(t, &models.Label{})
	}
}
