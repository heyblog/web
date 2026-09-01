package publicview

import (
	"context"
	"errors"
	"hash/fnv"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/apperror"
	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/domain/site"
)

const homeSiteLimit int32 = 6

type IdentifierKind uint8

const (
	IdentifierUUID IdentifierKind = iota + 1
	IdentifierShortID
)

type SiteIdentifier struct {
	Kind  IdentifierKind
	Value string
}

type Reader interface {
	Home(context.Context) (Home, error)
	SiteByIdentifier(context.Context, SiteIdentifier) (SiteProfile, error)
	SiteByCustomID(context.Context, string) (SiteProfile, error)
}

type Queries interface {
	CountVisibleSites(context.Context) (int64, error)
	GetLeadingActiveMainAnnouncement(context.Context) (dbgen.ContentAnnouncement, error)
	GetSiteByCustomID(context.Context, *string) (dbgen.DirectorySite, error)
	GetSiteByID(context.Context, pgtype.UUID) (dbgen.DirectorySite, error)
	GetSiteByShortID(context.Context, string) (dbgen.DirectorySite, error)
	ListPublicSiteFeeds(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteFeed, error)
	ListPublicSiteSoftwareComponents(context.Context, pgtype.UUID) ([]dbgen.ListPublicSiteSoftwareComponentsRow, error)
	ListPublicSiteTags(context.Context, pgtype.UUID) ([]dbgen.ListPublicSiteTagsRow, error)
	ListSiteResources(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteResource, error)
	ListVisibleSites(context.Context, dbgen.ListVisibleSitesParams) ([]dbgen.DirectorySite, error)
}

type Service struct {
	queries Queries
	now     func() time.Time
}

type Home struct {
	SiteCount    int64         `json:"siteCount"`
	Announcement *Announcement `json:"announcement"`
	Sites        []SiteCard    `json:"sites"`
}

type Announcement struct {
	Title    string              `json:"title"`
	StartsAt time.Time           `json:"startsAt"`
	Action   *AnnouncementAction `json:"action"`
}

type AnnouncementAction struct {
	Label    string `json:"label"`
	Href     string `json:"href"`
	External bool   `json:"external"`
}

type SiteCard struct {
	ShortID     string    `json:"shortId"`
	CustomID    *string   `json:"customId"`
	Name        string    `json:"name"`
	Summary     string    `json:"summary"`
	Host        string    `json:"host"`
	HomepageURL string    `json:"homepageUrl"`
	AccessScope string    `json:"accessScope"`
	JoinedAt    time.Time `json:"joinedAt"`
}

type SiteProfile struct {
	SiteCard
	UpdatedAt    time.Time    `json:"updatedAt"`
	Topics       []Topic      `json:"topics"`
	Warnings     []Warning    `json:"warnings"`
	Feeds        []Feed       `json:"feeds"`
	Resources    []Resource   `json:"resources"`
	Technologies []Technology `json:"technologies"`
}

type Topic struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Role        string `json:"role"`
}

type Warning struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type Feed struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Format    string `json:"format"`
	IsDefault bool   `json:"isDefault"`
}

type Resource struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

type Technology struct {
	Name          string  `json:"name"`
	Role          string  `json:"role"`
	HomepageURL   *string `json:"homepageUrl"`
	RepositoryURL *string `json:"repositoryUrl"`
	IsOpenSource  bool    `json:"isOpenSource"`
}

func New(queries Queries) *Service {
	return &Service{queries: queries, now: time.Now}
}

func (service *Service) Home(ctx context.Context) (Home, error) {
	count, err := service.queries.CountVisibleSites(ctx)
	if err != nil {
		return Home{}, internalError(err, "count visible sites")
	}
	if count < 0 || count > math.MaxInt32 {
		return Home{}, internalError(
			errors.New("visible site count is out of range"),
			"validate visible site count",
		)
	}

	sites, err := service.loadDailySites(ctx, count)
	if err != nil {
		return Home{}, err
	}
	announcement, err := service.loadAnnouncement(ctx)
	if err != nil {
		return Home{}, err
	}
	return Home{SiteCount: count, Announcement: announcement, Sites: sites}, nil
}

func (service *Service) SiteByIdentifier(
	ctx context.Context,
	identifier SiteIdentifier,
) (SiteProfile, error) {
	var (
		row dbgen.DirectorySite
		err error
	)
	switch identifier.Kind {
	case IdentifierUUID:
		var id pgtype.UUID
		if scanErr := id.Scan(identifier.Value); scanErr != nil || !id.Valid {
			return SiteProfile{}, badIdentifier("identifier")
		}
		row, err = service.queries.GetSiteByID(ctx, id)
	case IdentifierShortID:
		if validateErr := site.ValidateShortID(identifier.Value); validateErr != nil {
			return SiteProfile{}, badIdentifier("identifier")
		}
		row, err = service.queries.GetSiteByShortID(ctx, identifier.Value)
	default:
		return SiteProfile{}, badIdentifier("identifier")
	}
	return service.loadProfile(ctx, row, err)
}

func (service *Service) SiteByCustomID(ctx context.Context, customID string) (SiteProfile, error) {
	if err := site.ValidateCustomID(customID); err != nil {
		return SiteProfile{}, badIdentifier("customId")
	}
	row, err := service.queries.GetSiteByCustomID(ctx, &customID)
	return service.loadProfile(ctx, row, err)
}

func (service *Service) loadDailySites(ctx context.Context, count int64) ([]SiteCard, error) {
	if count == 0 {
		return []SiteCard{}, nil
	}
	limit := min(homeSiteLimit, int32(count)) // #nosec G115 -- Home validates count fits in int32.
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(service.now().UTC().Format(time.DateOnly)))
	offset := int32(uint64(hash.Sum32()) % uint64(count)) // #nosec G115 -- modulo count fits in int32.

	rows, err := service.queries.ListVisibleSites(
		ctx,
		dbgen.ListVisibleSitesParams{Limit: limit, Offset: offset},
	)
	if err != nil {
		return nil, internalError(err, "list daily visible sites")
	}
	if missing := limit - int32(len(rows)); missing > 0 && offset > 0 { // #nosec G115 -- rows cannot exceed the requested int32 limit.
		wrapped, wrapErr := service.queries.ListVisibleSites(
			ctx,
			dbgen.ListVisibleSitesParams{Limit: missing, Offset: 0},
		)
		if wrapErr != nil {
			return nil, internalError(wrapErr, "wrap daily visible sites")
		}
		rows = append(rows, wrapped...)
	}

	cards := make([]SiteCard, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if _, exists := seen[row.ShortID]; exists {
			continue
		}
		card, mapErr := mapSiteCard(row)
		if mapErr != nil {
			return nil, internalError(mapErr, "map daily visible site")
		}
		seen[row.ShortID] = struct{}{}
		cards = append(cards, card)
	}
	return cards, nil
}

func (service *Service) loadAnnouncement(ctx context.Context) (*Announcement, error) {
	row, err := service.queries.GetLeadingActiveMainAnnouncement(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, internalError(err, "load leading announcement")
	}
	if !row.StartsAt.Valid {
		return nil, internalError(
			errors.New("announcement start time is invalid"),
			"map leading announcement",
		)
	}
	return &Announcement{
		Title:    row.Title,
		StartsAt: row.StartsAt.Time,
		Action:   mapAnnouncementAction(row),
	}, nil
}

func (service *Service) loadProfile(
	ctx context.Context,
	row dbgen.DirectorySite,
	lookupErr error,
) (SiteProfile, error) {
	if errors.Is(lookupErr, pgx.ErrNoRows) || lookupErr == nil && row.Visibility != "VISIBLE" {
		return SiteProfile{}, notFound()
	}
	if lookupErr != nil {
		return SiteProfile{}, internalError(lookupErr, "load site profile")
	}

	card, err := mapSiteCard(row)
	if err != nil {
		return SiteProfile{}, internalError(err, "map site profile")
	}
	if !row.UpdatedAt.Valid {
		return SiteProfile{}, internalError(
			errors.New("site update time is invalid"),
			"map site profile",
		)
	}

	feeds, err := service.queries.ListPublicSiteFeeds(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site feeds")
	}
	resources, err := service.queries.ListSiteResources(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site resources")
	}
	tags, err := service.queries.ListPublicSiteTags(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site tags")
	}
	technologies, err := service.queries.ListPublicSiteSoftwareComponents(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site technologies")
	}

	address := site.Address{
		Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath,
	}
	profile := SiteProfile{
		SiteCard:     card,
		UpdatedAt:    row.UpdatedAt.Time,
		Topics:       []Topic{},
		Warnings:     []Warning{},
		Feeds:        make([]Feed, 0, len(feeds)),
		Resources:    make([]Resource, 0, len(resources)),
		Technologies: make([]Technology, 0, len(technologies)),
	}
	for _, tag := range tags {
		if tag.Role == "WARNING" {
			profile.Warnings = append(profile.Warnings, Warning{
				Name: tag.Name, Slug: tag.Slug, Description: tag.Description,
			})
			continue
		}
		profile.Topics = append(profile.Topics, Topic{
			Name: tag.Name, Slug: tag.Slug, Description: tag.Description, Role: tag.Role,
		})
	}
	for _, feed := range feeds {
		locationURL, mapErr := address.LocationURL(
			locationFromValues(feed.LocationType, feed.UrlRef, feed.ExternalUrl),
		)
		if mapErr != nil {
			return SiteProfile{}, internalError(mapErr, "map public site feed")
		}
		profile.Feeds = append(profile.Feeds, Feed{
			Name: feed.Name, URL: locationURL, Format: feed.Format, IsDefault: feed.IsDefault,
		})
	}
	for _, resource := range resources {
		locationURL, mapErr := address.LocationURL(
			locationFromValues(resource.LocationType, resource.UrlRef, resource.ExternalUrl),
		)
		if mapErr != nil {
			return SiteProfile{}, internalError(mapErr, "map public site resource")
		}
		profile.Resources = append(profile.Resources, Resource{Kind: resource.Kind, URL: locationURL})
	}
	for _, technology := range technologies {
		profile.Technologies = append(profile.Technologies, Technology{
			Name:          technology.Name,
			Role:          technology.Role,
			HomepageURL:   technology.HomepageUrl,
			RepositoryURL: technology.RepositoryUrl,
			IsOpenSource:  technology.IsOpenSource,
		})
	}
	return profile, nil
}

func mapSiteCard(row dbgen.DirectorySite) (SiteCard, error) {
	if !row.JoinedAt.Valid {
		return SiteCard{}, errors.New("site join time is invalid")
	}
	homepageURL, err := (site.Address{
		Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath,
	}).HomepageURL()
	if err != nil {
		return SiteCard{}, err
	}
	return SiteCard{
		ShortID:     row.ShortID,
		CustomID:    row.CustomID,
		Name:        row.Name,
		Summary:     row.Summary,
		Host:        row.NormalizedHost,
		HomepageURL: homepageURL,
		AccessScope: row.AccessScope,
		JoinedAt:    row.JoinedAt.Time,
	}, nil
}

func mapAnnouncementAction(row dbgen.ContentAnnouncement) *AnnouncementAction {
	switch row.ActionType {
	case "INTERNAL":
		if row.ActionLabel != nil && row.ActionPath != nil {
			return &AnnouncementAction{Label: *row.ActionLabel, Href: *row.ActionPath}
		}
	case "EXTERNAL":
		if row.ActionLabel != nil && row.ActionExternalUrl != nil {
			return &AnnouncementAction{
				Label: *row.ActionLabel, Href: *row.ActionExternalUrl, External: true,
			}
		}
	}
	return nil
}

func locationFromValues(locationType string, urlRef, externalURL *string) site.Location {
	location := site.Location{Type: locationType}
	if urlRef != nil {
		location.URLRef = *urlRef
	}
	if externalURL != nil {
		location.ExternalURL = *externalURL
	}
	return location
}

func badIdentifier(name string) error {
	return apperror.New(
		apperror.KindBadRequest,
		apperror.CodeBadRequest,
		"site identifier is invalid",
	).WithInvalidParams([]apperror.InvalidParam{{
		Name: name, Reason: "must use the accepted route format",
	}})
}

func notFound() error {
	return apperror.New(apperror.KindNotFound, apperror.CodeNotFound, "site was not found")
}

func internalError(err error, operation string) error {
	return apperror.Wrap(
		err,
		apperror.KindInternal,
		apperror.CodeInternal,
		"unable to load public site data",
		operation,
	)
}

var _ Reader = (*Service)(nil)
