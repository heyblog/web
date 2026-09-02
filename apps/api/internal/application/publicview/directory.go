package publicview

import (
	"context"
	"errors"
	"time"

	dbgen "heyblog-api/internal/database/gen"
)

const directoryPageSize int32 = 24
const directoryMaximumPages int64 = 100_000

type DirectoryFeed string

const (
	DirectoryFeedAny     DirectoryFeed = "any"
	DirectoryFeedWith    DirectoryFeed = "with"
	DirectoryFeedWithout DirectoryFeed = "without"
)

type DirectorySort string

const (
	DirectorySortRandom  DirectorySort = "random"
	DirectorySortJoined  DirectorySort = "joined"
	DirectorySortUpdated DirectorySort = "updated"
)

type DirectoryOrder string

const (
	DirectoryOrderAscending  DirectoryOrder = "asc"
	DirectoryOrderDescending DirectoryOrder = "desc"
)

type DirectoryQuery struct {
	Page          int32           `json:"page"`
	Query         string          `json:"q"`
	PrimaryTags   []string        `json:"primary"`
	SecondaryTags []string        `json:"secondary"`
	Warnings      []string        `json:"warning"`
	Technologies  []string        `json:"technology"`
	AccessScopes  []string        `json:"access"`
	Feed          DirectoryFeed   `json:"feed"`
	Status        DirectoryStatus `json:"status"`
	Sort          DirectorySort   `json:"sort"`
	Order         DirectoryOrder  `json:"order"`
	Seed          string          `json:"seed"`
}

type DirectoryView struct {
	Items        []SiteCardView        `json:"items"`
	Pagination   DirectoryPagination   `json:"pagination"`
	Query        DirectoryQuery        `json:"query"`
	StatusCounts DirectoryStatusCounts `json:"statusCounts"`
}

type DirectoryPagination struct {
	Page       int32 `json:"page"`
	PageSize   int32 `json:"pageSize"`
	TotalItems int64 `json:"totalItems"`
	TotalPages int32 `json:"totalPages"`
}

type directoryDatabaseParameters struct {
	SiteVisibility    string
	QueryText         string
	PrimaryTagSlugs   []string
	SecondaryTagSlugs []string
	WarningSlugs      []string
	TechnologyNames   []string
	AccessScopes      []string
	FeedMode          string
	SortMode          string
	Seed              string
	SortOrder         string
	Offset            int32
	Limit             int32
	Page              int32
}

func DefaultDirectoryQuery(now time.Time) DirectoryQuery {
	chinaTime := now.In(time.FixedZone("China Standard Time", 8*60*60))
	return DirectoryQuery{
		Page: 1, PrimaryTags: []string{}, SecondaryTags: []string{}, Warnings: []string{},
		Technologies: []string{}, AccessScopes: []string{}, Feed: DirectoryFeedAny,
		Status: DirectoryStatusNormal, Sort: DirectorySortRandom,
		Order: DirectoryOrderDescending, Seed: "site-directory:" + chinaTime.Format(time.DateOnly),
	}
}

func (service *Service) Directory(ctx context.Context, query DirectoryQuery) (DirectoryView, error) {
	countParameters := query.countParameters()
	counts, err := service.queries.CountDirectorySitesByStatus(ctx, countParameters)
	if err != nil {
		return DirectoryView{}, internalError(err, "count directory sites")
	}
	if counts.NormalCount < 0 || counts.AbnormalCount < 0 {
		return DirectoryView{}, internalError(
			errors.New("directory site count is out of range"),
			"validate directory site count",
		)
	}
	visibility, err := visibilityFromDirectoryStatus(query.Status)
	if err != nil {
		return DirectoryView{}, internalError(err, "map directory status")
	}
	statusCounts := DirectoryStatusCounts{Normal: counts.NormalCount, Abnormal: counts.AbnormalCount}
	totalItems := statusCounts.Normal
	if query.Status == DirectoryStatusAbnormal {
		totalItems = statusCounts.Abnormal
	}

	parameters := query.databaseParameters(totalItems, visibility)
	rows, err := service.queries.ListDirectorySites(ctx, parameters.listParameters())
	if err != nil {
		return DirectoryView{}, internalError(err, "list directory sites")
	}
	cards, err := service.loadSiteCards(ctx, rows)
	if err != nil {
		return DirectoryView{}, err
	}
	query.Page = parameters.Page
	return DirectoryView{
		Items: cards,
		Pagination: DirectoryPagination{
			Page: parameters.Page, PageSize: directoryPageSize, TotalItems: totalItems,
			TotalPages: directoryTotalPages(totalItems),
		},
		Query: query, StatusCounts: statusCounts,
	}, nil
}

func (query DirectoryQuery) countParameters() dbgen.CountDirectorySitesByStatusParams {
	return dbgen.CountDirectorySitesByStatusParams{
		QueryText: query.Query, PrimaryTagSlugs: query.PrimaryTags,
		SecondaryTagSlugs: query.SecondaryTags, WarningSlugs: query.Warnings,
		TechnologyNames: query.Technologies, AccessScopes: query.AccessScopes, FeedMode: string(query.Feed),
	}
}

func (query DirectoryQuery) databaseParameters(totalItems int64, visibility string) directoryDatabaseParameters {
	page := query.Page
	totalPages := directoryTotalPages(totalItems)
	if page > totalPages {
		page = totalPages
	}
	return directoryDatabaseParameters{
		SiteVisibility: visibility, QueryText: query.Query, PrimaryTagSlugs: query.PrimaryTags,
		SecondaryTagSlugs: query.SecondaryTags, WarningSlugs: query.Warnings,
		TechnologyNames: query.Technologies, AccessScopes: query.AccessScopes, FeedMode: string(query.Feed),
		SortMode: string(query.Sort), Seed: query.Seed,
		SortOrder: string(query.Order), Offset: (page - 1) * directoryPageSize,
		Limit: directoryPageSize, Page: page,
	}
}

func (parameters directoryDatabaseParameters) listParameters() dbgen.ListDirectorySitesParams {
	return dbgen.ListDirectorySitesParams{
		SiteVisibility: parameters.SiteVisibility, QueryText: parameters.QueryText,
		PrimaryTagSlugs: parameters.PrimaryTagSlugs, SecondaryTagSlugs: parameters.SecondaryTagSlugs,
		WarningSlugs: parameters.WarningSlugs, TechnologyNames: parameters.TechnologyNames,
		AccessScopes: parameters.AccessScopes, FeedMode: parameters.FeedMode,
		SortMode: parameters.SortMode, Seed: parameters.Seed, SortOrder: parameters.SortOrder,
		PageOffset: parameters.Offset, PageLimit: parameters.Limit,
	}
}

func directoryTotalPages(totalItems int64) int32 {
	if totalItems == 0 {
		return 1
	}
	pageCount := (totalItems + int64(directoryPageSize) - 1) / int64(directoryPageSize)
	if pageCount > directoryMaximumPages {
		return int32(directoryMaximumPages)
	}
	return int32(pageCount) // #nosec G115 -- pageCount is bounded by directoryMaximumPages.
}
