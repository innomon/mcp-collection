package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SearchCatalogInput struct {
	Query    string  `json:"query"`
	Context  string  `json:"context"`
	Category string  `json:"category,omitempty"`
	MinPrice float64 `json:"min_price,omitempty"`
	MaxPrice float64 `json:"max_price,omitempty"`
	PerPage  int     `json:"per_page,omitempty"`
}

type GetProductInput struct {
	ID string `json:"id"`
}

type SearchPoliciesInput struct {
	Query string `json:"query"`
}

func handleSearchShopCatalog(client *WooClient) func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args SearchCatalogInput
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parse search catalog input: %w", err)
		}

		if args.PerPage == 0 {
			args.PerPage = 10
		}

		params := SearchParams{
			Query:    args.Query,
			Category: args.Category,
			PerPage:  args.PerPage,
		}
		if args.MinPrice != 0 {
			params.MinPrice = fmt.Sprintf("%.2f", args.MinPrice)
		}
		if args.MaxPrice != 0 {
			params.MaxPrice = fmt.Sprintf("%.2f", args.MaxPrice)
		}

		products, err := client.SearchProductsAdvanced(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("search shop catalog: %w", err)
		}

		type catalogResult struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Price       int    `json:"price"`
			Currency    string `json:"currency"`
			ImageURL    string `json:"image_url"`
			URL         string `json:"url"`
			Description string `json:"description"`
			VariantID   string `json:"variant_id"`
			InStock     bool   `json:"in_stock"`
		}

		results := make([]catalogResult, 0, len(products))
		for _, p := range products {
			price, _ := wcPriceToCents(p.Price)
			imageURL := ""
			if len(p.Images) > 0 {
				imageURL = p.Images[0].Src
			}
			inStock := p.StockStatus == "instock" || p.StockStatus == ""
			results = append(results, catalogResult{
				ID:          strconv.Itoa(p.ID),
				Title:       p.Name,
				Price:       price,
				Currency:    "USD",
				ImageURL:    imageURL,
				URL:         p.Permalink,
				Description: p.Description,
				VariantID:   strconv.Itoa(p.ID),
				InStock:     inStock,
			})
		}

		data, err := json.Marshal(results)
		if err != nil {
			return nil, fmt.Errorf("marshal catalog results: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil
	}
}

func handleGetProduct(client *WooClient) func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args GetProductInput
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parse get product input: %w", err)
		}

		id, err := strconv.Atoi(args.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid product id %q: %w", args.ID, err)
		}

		product, err := client.GetProduct(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get product: %w", err)
		}

		price, _ := wcPriceToCents(product.Price)
		imageURL := ""
		if len(product.Images) > 0 {
			imageURL = product.Images[0].Src
		}

		type categoryInfo struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		type variantInfo struct {
			ID         string            `json:"id"`
			Title      string            `json:"title"`
			Price      int               `json:"price"`
			InStock    bool              `json:"in_stock"`
			Attributes map[string]string `json:"attributes"`
		}
		type productResult struct {
			ID          string         `json:"id"`
			Title       string         `json:"title"`
			Price       int            `json:"price"`
			Currency    string         `json:"currency"`
			ImageURL    string         `json:"image_url"`
			URL         string         `json:"url"`
			Description string         `json:"description"`
			Categories  []categoryInfo `json:"categories"`
			Variants    []variantInfo  `json:"variants,omitempty"`
		}

		cats := make([]categoryInfo, 0, len(product.Categories))
		for _, c := range product.Categories {
			cats = append(cats, categoryInfo{ID: c.ID, Name: c.Name})
		}

		var variants []variantInfo
		if product.Type == "variable" && len(product.Variations) > 0 {
			variations, err := client.GetProductVariations(ctx, product.ID)
			if err != nil {
				return nil, fmt.Errorf("get product variations: %w", err)
			}
			variants = make([]variantInfo, 0, len(variations))
			for _, v := range variations {
				vPrice, _ := wcPriceToCents(v.Price)
				attrs := make(map[string]string, len(v.Attributes))
				titleParts := make([]string, 0, len(v.Attributes))
				for _, a := range v.Attributes {
					attrs[a.Name] = a.Option
					titleParts = append(titleParts, a.Option)
				}
				title := product.Name
				if len(titleParts) > 0 {
					title = product.Name + " - " + strings.Join(titleParts, ", ")
				}
				inStock := v.StockStatus == "instock" || v.StockStatus == ""
				variants = append(variants, variantInfo{
					ID:         strconv.Itoa(v.ID),
					Title:      title,
					Price:      vPrice,
					InStock:    inStock,
					Attributes: attrs,
				})
			}
		}

		result := productResult{
			ID:          strconv.Itoa(product.ID),
			Title:       product.Name,
			Price:       price,
			Currency:    "USD",
			ImageURL:    imageURL,
			URL:         product.Permalink,
			Description: product.Description,
			Categories:  cats,
			Variants:    variants,
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal product result: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil
	}
}

func handleGetProductCategories(client *WooClient) func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		categories, err := client.GetProductCategories(ctx)
		if err != nil {
			return nil, fmt.Errorf("get product categories: %w", err)
		}

		type categoryEntry struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Slug string `json:"slug"`
		}

		type categoriesResult struct {
			Categories []categoryEntry `json:"categories"`
		}

		entries := make([]categoryEntry, 0, len(categories))
		for _, c := range categories {
			entries = append(entries, categoryEntry{ID: c.ID, Name: c.Name, Slug: c.Slug})
		}

		result := categoriesResult{Categories: entries}
		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal categories result: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil
	}
}

func handleSearchShopPolicies(client *WooClient) func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args SearchPoliciesInput
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parse search policies input: %w", err)
		}

		pages, err := client.GetPages(ctx, args.Query)
		if err != nil {
			return nil, fmt.Errorf("search shop policies: %w", err)
		}

		type pageEntry struct {
			ID      int    `json:"id"`
			Title   string `json:"title"`
			Content string `json:"content"`
			Slug    string `json:"slug"`
		}

		type pagesResult struct {
			Pages []pageEntry `json:"pages"`
		}

		entries := make([]pageEntry, 0, len(pages))
		for _, p := range pages {
			entries = append(entries, pageEntry{
				ID:      p.ID,
				Title:   p.Title.Rendered,
				Content: p.Content.Rendered,
				Slug:    p.Slug,
			})
		}

		result := pagesResult{Pages: entries}
		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal policies result: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil
	}
}


