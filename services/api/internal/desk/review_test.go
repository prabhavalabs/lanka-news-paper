package desk

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateArticleReviewAcceptsEditorialDecision(t *testing.T) {
	category := "politics"
	review := ArticleReview{Status: "published", Category: &category}

	err := validateArticleReview(review)

	require.NoError(t, err)
}

func TestValidateArticleReviewRejectsUnsupportedStatus(t *testing.T) {
	review := ArticleReview{Status: "draft"}

	err := validateArticleReview(review)

	require.EqualError(t, err, "status has an unsupported value")
}

func TestValidateArticleReviewRejectsEmptyCategory(t *testing.T) {
	category := ""
	review := ArticleReview{Category: &category}

	err := validateArticleReview(review)

	require.EqualError(t, err, "category is required when provided")
}

func TestValidateArticleReviewRejectsEmptyDecision(t *testing.T) {
	err := validateArticleReview(ArticleReview{})

	require.EqualError(t, err, "status or category is required")
}
