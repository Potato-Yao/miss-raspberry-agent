package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"miss-raspberry-agent/internal/adapter/http/dto"
	"miss-raspberry-agent/internal/tagging"
)

// TaggingService is the application service the tagging handler depends on.
type TaggingService interface {
	RegisterSet(ctx context.Context, set tagging.TagSet) error
	Tag(ctx context.Context, setName, text string) (*tagging.Match, error)
}

// TaggingHandler serves the tag-set registration and tagging endpoints.
type TaggingHandler struct {
	service TaggingService
}

// NewTaggingHandler creates a TaggingHandler backed by the given service.
func NewTaggingHandler(service TaggingService) *TaggingHandler {
	return &TaggingHandler{service: service}
}

// RegisterSet handles POST /api/v1/agents/tagger/tag-sets: it validates the body,
// stores the set (replacing any set with the same name), and returns 200 OK.
func (h *TaggingHandler) RegisterSet(c *gin.Context) {
	var req dto.RegisterTagSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body: " + err.Error()})
		return
	}

	tags := make([]tagging.Tag, 0, len(req.Tags))
	for _, t := range req.Tags {
		tags = append(tags, tagging.Tag{
			Name:        t.Name,
			Description: t.Description,
			ApplyRule:   t.ApplyRule,
		})
	}

	set := tagging.TagSet{Name: req.Name, Prompt: req.Prompt, Tags: tags}
	if err := h.service.RegisterSet(c.Request.Context(), set); err != nil {
		switch {
		case errors.Is(err, tagging.ErrInvalidTagSet), errors.Is(err, tagging.ErrDuplicateTagName):
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		default:
			_ = c.Error(err)
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, dto.RegisterTagSetResponse{Name: req.Name, TagCount: len(tags)})
}

// Tag handles POST /api/v1/agents/tagger/tag: it validates the body, confirms
// the tag set exists, and returns the single best-matching tag with the tagger's
// reason. The "tag" field is null when no tag applies.
func (h *TaggingHandler) Tag(c *gin.Context) {
	var req dto.TagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body: " + err.Error()})
		return
	}

	match, err := h.service.Tag(c.Request.Context(), req.Name, req.Text)
	if err != nil {
		switch {
		case errors.Is(err, tagging.ErrTagSetNotFound):
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
		case errors.Is(err, tagging.ErrInvalidTagSet):
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		default:
			_ = c.Error(err)
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal error"})
		}
		return
	}

	resp := dto.TagResponse{Name: req.Name}
	if match != nil {
		resp.Tag = &dto.TagItem{
			Name:   match.Tag.Name,
			Reason: match.Reason,
		}
	}
	c.JSON(http.StatusOK, resp)
}
