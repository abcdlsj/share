package notion

import (
	"context"
	"fmt"
	"time"

	"notionbot/internal/model"

	"github.com/jomei/notionapi"
)

type Writer struct {
	client          *notionapi.Client
	database        notionapi.DatabaseID
	titleProp       string
	createdProp     string
	visibilityProp  string
	visibilityValue string
	loc             *time.Location
}

func NewWriter(token, databaseID, titleProp, createdProp, visibilityProp, visibilityValue string, loc *time.Location) *Writer {
	client := notionapi.NewClient(notionapi.Token(token))
	return &Writer{
		client:          client,
		database:        notionapi.DatabaseID(databaseID),
		titleProp:       titleProp,
		createdProp:     createdProp,
		visibilityProp:  visibilityProp,
		visibilityValue: visibilityValue,
		loc:             loc,
	}
}

func (n *Writer) CreateNotePage(ctx context.Context, title string, entries []model.Entry) (notionapi.PageID, string, error) {
	blocks := entriesToBlocks(entries)
	first, rest := splitBlocks(blocks, 50)

	props := notionapi.Properties{}
	props[n.titleProp] = notionapi.TitleProperty{
		Title: []notionapi.RichText{{Text: &notionapi.Text{Content: title}}},
	}

	// Optional fixed properties for your DB.
	if n.createdProp != "" {
		now := time.Now()
		if n.loc != nil {
			now = now.In(n.loc)
		}
		d := notionapi.Date(now)
		props[n.createdProp] = notionapi.DateProperty{Date: &notionapi.DateObject{Start: &d}}
	}
	if n.visibilityProp != "" && n.visibilityValue != "" {
		props[n.visibilityProp] = notionapi.SelectProperty{Select: notionapi.Option{Name: n.visibilityValue}}
	}

	resp, err := n.client.Page.Create(ctx, &notionapi.PageCreateRequest{
		Parent:     notionapi.Parent{DatabaseID: n.database},
		Properties: props,
		Children:   first,
	})
	if err != nil {
		return "", "", err
	}

	pageID := notionapi.PageID(resp.ID)
	pageURL := resp.URL

	for len(rest) > 0 {
		batch, next := splitBlocks(rest, 50)
		rest = next
		_, err = n.client.Block.AppendChildren(ctx, notionapi.BlockID(resp.ID), &notionapi.AppendBlockChildrenRequest{Children: batch})
		if err != nil {
			return pageID, pageURL, fmt.Errorf("append blocks: %w", err)
		}
	}

	return pageID, pageURL, nil
}

func entriesToBlocks(entries []model.Entry) []notionapi.Block {
	blocks := make([]notionapi.Block, 0, len(entries))
	for _, e := range entries {
		switch e.Type {
		case model.EntryText:
			if e.Text == "" {
				continue
			}
			blocks = append(blocks, notionapi.ParagraphBlock{
				BasicBlock: notionapi.BasicBlock{Object: "block", Type: notionapi.BlockTypeParagraph},
				Paragraph: notionapi.Paragraph{
					RichText: []notionapi.RichText{{Text: &notionapi.Text{Content: e.Text}}},
				},
			})
		case model.EntryImage:
			if e.URL == "" {
				continue
			}
			blocks = append(blocks, notionapi.ImageBlock{
				BasicBlock: notionapi.BasicBlock{Object: "block", Type: notionapi.BlockTypeImage},
				Image: notionapi.Image{
					Type:     notionapi.FileTypeExternal,
					External: &notionapi.FileObject{URL: e.URL},
				},
			})
		}
	}
	return blocks
}

func splitBlocks(in []notionapi.Block, n int) (head []notionapi.Block, tail []notionapi.Block) {
	if len(in) <= n {
		return in, nil
	}
	return in[:n], in[n:]
}
