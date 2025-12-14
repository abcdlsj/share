package notion

import (
	"context"
	"fmt"

	"notionbot/internal/model"

	"github.com/jomei/notionapi"
)

type Writer struct {
	client    *notionapi.Client
	database  notionapi.DatabaseID
	titleProp string
}

func NewWriter(token, databaseID, titleProp string) *Writer {
	client := notionapi.NewClient(notionapi.Token(token))
	return &Writer{
		client:    client,
		database:  notionapi.DatabaseID(databaseID),
		titleProp: titleProp,
	}
}

func (n *Writer) CreateNotePage(ctx context.Context, title string, entries []model.Entry) (notionapi.PageID, string, error) {
	blocks := entriesToBlocks(entries)
	first, rest := splitBlocks(blocks, 50)

	props := notionapi.Properties{
		n.titleProp: notionapi.TitleProperty{
			Title: []notionapi.RichText{{Text: &notionapi.Text{Content: title}}},
		},
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
				BasicBlock: notionapi.BasicBlock{Type: notionapi.BlockTypeParagraph},
				Paragraph: notionapi.Paragraph{
					RichText: []notionapi.RichText{{Text: &notionapi.Text{Content: e.Text}}},
				},
			})
		case model.EntryImage:
			if e.URL == "" {
				continue
			}
			blocks = append(blocks, notionapi.ImageBlock{
				BasicBlock: notionapi.BasicBlock{Type: notionapi.BlockTypeImage},
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
