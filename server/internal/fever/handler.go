// Package fever implements the Fever API compatibility layer,
// allowing third-party RSS clients like Reeder and NetNewsWire to connect.
//
// The Fever API is documented at https://feedafever.com/api
// All requests are POST to /fever/ with query parameters determining what data to return.
// Authentication uses api_key in the POST form body.
package fever

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/internal/middleware"
)

// Handler implements the Fever API compatibility endpoint.
type Handler struct {
	pool *pgxpool.Pool
}

// NewHandler creates a new Fever API handler.
func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

// HashFeverKey computes SHA-256 of the given api_key string and returns a hex-encoded result.
// The Fever protocol sends api_key = MD5(username:password).
// We store SHA-256(api_key) in the database, never the raw MD5.
func HashFeverKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}

// ComputeFeverAPIKey computes the raw Fever API key: MD5(username:password).
func ComputeFeverAPIKey(username, password string) string {
	h := md5.Sum([]byte(username + ":" + password))
	return hex.EncodeToString(h[:])
}

type feverUser struct {
	ID             int64
	GitHubUsername string
}

// authenticate checks the api_key form field and returns the authenticated user, or nil.
func (h *Handler) authenticate(c *gin.Context) *feverUser {
	apiKey := c.PostForm("api_key")
	if apiKey == "" {
		return nil
	}

	hashedKey := HashFeverKey(apiKey)

	var u feverUser
	err := h.pool.QueryRow(c.Request.Context(),
		"SELECT id, github_username FROM users WHERE fever_api_key = $1",
		hashedKey,
	).Scan(&u.ID, &u.GitHubUsername)
	if err != nil {
		return nil
	}
	return &u
}

// Handle is the single endpoint for all Fever API requests: POST /fever/
func (h *Handler) Handle(c *gin.Context) {
	// The Fever API always returns JSON with api_version and auth fields.
	base := gin.H{
		"api_version": 3,
		"auth":        0,
	}

	user := h.authenticate(c)
	if user == nil {
		// Even on auth failure, Fever expects a 200 with auth:0.
		c.JSON(http.StatusOK, base)
		return
	}
	base["auth"] = 1
	base["last_refreshed_on_time"] = time.Now().Unix()

	ctx := c.Request.Context()

	// Check what data the client is requesting via query parameters.
	query := c.Request.URL.Query()

	if query.Has("feeds") || query.Has("groups") {
		h.handleFeedsAndGroups(c, user, base)
	}

	if query.Has("favicons") {
		// Return a minimal favicon list. Fever clients expect this.
		type feverFavicon struct {
			ID   int64  `json:"id"`
			Data string `json:"data"`
		}
		base["favicons"] = []feverFavicon{}
	}

	if query.Has("items") {
		h.handleItems(c, user, base)
	}

	if query.Has("unread_item_ids") {
		h.handleUnreadIDs(c, user, base)
	}

	if query.Has("saved_item_ids") {
		h.handleSavedIDs(c, user, base)
	}

	// Handle mark actions
	mark := c.PostForm("mark")
	as := c.PostForm("as")
	if mark != "" && as != "" {
		h.handleMark(c, user, mark, as)
	}

	c.JSON(http.StatusOK, base)
	_ = ctx // suppress unused
}

func (h *Handler) handleFeedsAndGroups(c *gin.Context, user *feverUser, base gin.H) {
	ctx := c.Request.Context()
	query := c.Request.URL.Query()

	rows, err := h.pool.Query(ctx,
		"SELECT id, url, title, icon_url, category FROM sources WHERE user_id = $1 AND deleted_at IS NULL ORDER BY id",
		user.ID,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	type feverFeed struct {
		ID          int64  `json:"id"`
		FaviconID   int64  `json:"favicon_id"`
		Title       string `json:"title"`
		URL         string `json:"url"`
		SiteURL     string `json:"site_url"`
		IsSpark     int    `json:"is_spark"`
		LastUpdated int64  `json:"last_updated_on_time"`
	}

	feeds := []feverFeed{}
	// Use ordered slice of category names to produce stable group IDs
	var categoryOrder []string
	groupSet := map[string][]int64{} // category -> source IDs

	for rows.Next() {
		var id int64
		var url, title, category string
		var iconURL *string
		if err := rows.Scan(&id, &url, &title, &iconURL, &category); err != nil {
			continue
		}
		feeds = append(feeds, feverFeed{
			ID:          id,
			FaviconID:   id,
			Title:       title,
			URL:         url,
			SiteURL:     url,
			IsSpark:     0,
			LastUpdated: time.Now().Unix(),
		})
		if category == "" {
			category = "General"
		}
		if _, seen := groupSet[category]; !seen {
			categoryOrder = append(categoryOrder, category)
		}
		groupSet[category] = append(groupSet[category], id)
	}
	if err := rows.Err(); err != nil {
		return
	}

	type feedsGroup struct {
		GroupID int64  `json:"group_id"`
		FeedIDs string `json:"feed_ids"`
	}

	buildFeedsGroups := func() []feedsGroup {
		fgs := []feedsGroup{}
		for i, cat := range categoryOrder {
			ids := groupSet[cat]
			strIDs := make([]string, len(ids))
			for j, id := range ids {
				strIDs[j] = strconv.FormatInt(id, 10)
			}
			fgs = append(fgs, feedsGroup{
				GroupID: int64(i + 1),
				FeedIDs: strings.Join(strIDs, ","),
			})
		}
		return fgs
	}

	if query.Has("feeds") {
		base["feeds"] = feeds
		base["feeds_groups"] = buildFeedsGroups()
	}

	if query.Has("groups") {
		type feverGroup struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		}
		groups := []feverGroup{}
		for i, cat := range categoryOrder {
			groups = append(groups, feverGroup{ID: int64(i + 1), Title: cat})
		}
		base["groups"] = groups
		base["feeds_groups"] = buildFeedsGroups()
	}
}

func (h *Handler) handleItems(c *gin.Context, user *feverUser, base gin.H) {
	ctx := c.Request.Context()
	query := c.Request.URL.Query()

	sinceID, _ := strconv.ParseInt(query.Get("since_id"), 10, 64)
	maxID, _ := strconv.ParseInt(query.Get("max_id"), 10, 64)
	withIDs := query.Get("with_ids")

	type feverItem struct {
		ID            int64  `json:"id"`
		FeedID        int64  `json:"feed_id"`
		Title         string `json:"title"`
		Author        string `json:"author"`
		HTML          string `json:"html"`
		URL           string `json:"url"`
		IsSaved       int    `json:"is_saved"`
		IsRead        int    `json:"is_read"`
		CreatedOnTime int64  `json:"created_on_time"`
	}

	items := []feverItem{}

	scanItems := func(rows interface {
		Next() bool
		Scan(dest ...interface{}) error
	}) {
		for rows.Next() {
			var item feverItem
			var publishedAt time.Time
			var isRead, isSaved bool
			if err := rows.Scan(&item.ID, &item.FeedID, &item.Title, &item.Author,
				&item.HTML, &item.URL, &publishedAt, &isRead, &isSaved); err != nil {
				continue
			}
			if isRead {
				item.IsRead = 1
			}
			if isSaved {
				item.IsSaved = 1
			}
			item.CreatedOnTime = publishedAt.Unix()
			items = append(items, item)
		}
	}

	if withIDs != "" {
		idStrs := strings.Split(withIDs, ",")
		ids := make([]int64, 0, len(idStrs))
		for _, s := range idStrs {
			id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err == nil {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			rows, err := h.pool.Query(ctx,
				`SELECT a.id, a.source_id, a.title, COALESCE(a.author, ''), a.content_html, a.link, a.published_at,
				        COALESCE(st.is_read, false), COALESCE(st.is_starred, false)
				 FROM articles a
				 JOIN sources s ON a.source_id = s.id
				 LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
				 WHERE s.user_id = $1 AND s.deleted_at IS NULL
				   AND a.id = ANY($2)
				 ORDER BY a.id DESC`,
				user.ID, ids,
			)
			if err == nil {
				defer rows.Close()
				scanItems(rows)
				if err := rows.Err(); err != nil {
					return
				}
			}
		}
	} else {
		rows, err := h.pool.Query(ctx,
			`SELECT a.id, a.source_id, a.title, COALESCE(a.author, ''), a.content_html, a.link, a.published_at,
			        COALESCE(st.is_read, false), COALESCE(st.is_starred, false)
			 FROM articles a
			 JOIN sources s ON a.source_id = s.id
			 LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
			 WHERE s.user_id = $1 AND s.deleted_at IS NULL
			   AND ($2::bigint = 0 OR a.id > $2)
			   AND ($3::bigint = 0 OR a.id < $3)
			 ORDER BY a.id DESC
			 LIMIT 50`,
			user.ID, sinceID, maxID,
		)
		if err == nil {
			defer rows.Close()
			scanItems(rows)
			if err := rows.Err(); err != nil {
				return
			}
		}
	}

	base["items"] = items
	base["total_items"] = len(items)
}

func (h *Handler) handleUnreadIDs(c *gin.Context, user *feverUser, base gin.H) {
	ctx := c.Request.Context()
	rows, err := h.pool.Query(ctx,
		`SELECT a.id
		 FROM articles a
		 JOIN sources s ON a.source_id = s.id
		 LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
		 WHERE s.user_id = $1 AND s.deleted_at IS NULL
		   AND (st.is_read IS NULL OR st.is_read = false)
		 ORDER BY a.id`,
		user.ID,
	)
	if err != nil {
		base["unread_item_ids"] = ""
		return
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
	}
	if err := rows.Err(); err != nil {
		base["unread_item_ids"] = ""
		return
	}
	base["unread_item_ids"] = strings.Join(ids, ",")
}

func (h *Handler) handleSavedIDs(c *gin.Context, user *feverUser, base gin.H) {
	ctx := c.Request.Context()
	rows, err := h.pool.Query(ctx,
		`SELECT a.id
		 FROM articles a
		 JOIN sources s ON a.source_id = s.id
		 JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
		 WHERE s.user_id = $1 AND s.deleted_at IS NULL
		   AND st.is_starred = true
		 ORDER BY a.id`,
		user.ID,
	)
	if err != nil {
		base["saved_item_ids"] = ""
		return
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
	}
	if err := rows.Err(); err != nil {
		base["saved_item_ids"] = ""
		return
	}
	base["saved_item_ids"] = strings.Join(ids, ",")
}

func (h *Handler) handleMark(c *gin.Context, user *feverUser, mark, as string) {
	ctx := c.Request.Context()
	id := c.PostForm("id")

	switch mark {
	case "item":
		articleID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return
		}
		switch as {
		case "read":
			_, _ = h.pool.Exec(ctx,
				`INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
				 SELECT $1, a.id, true, now()
				 FROM articles a JOIN sources s ON a.source_id = s.id
				 WHERE a.id = $2 AND s.user_id = $1 AND s.deleted_at IS NULL
				 ON CONFLICT (user_id, article_id) DO UPDATE SET is_read = true, last_read_at = now()`,
				user.ID, articleID,
			)
		case "unread":
			_, _ = h.pool.Exec(ctx,
				`INSERT INTO article_states (user_id, article_id, is_read)
				 SELECT $1, a.id, false
				 FROM articles a JOIN sources s ON a.source_id = s.id
				 WHERE a.id = $2 AND s.user_id = $1 AND s.deleted_at IS NULL
				 ON CONFLICT (user_id, article_id) DO UPDATE SET is_read = false`,
				user.ID, articleID,
			)
		case "saved":
			_, _ = h.pool.Exec(ctx,
				`INSERT INTO article_states (user_id, article_id, is_starred)
				 SELECT $1, a.id, true
				 FROM articles a JOIN sources s ON a.source_id = s.id
				 WHERE a.id = $2 AND s.user_id = $1 AND s.deleted_at IS NULL
				 ON CONFLICT (user_id, article_id) DO UPDATE SET is_starred = true`,
				user.ID, articleID,
			)
		case "unsaved":
			_, _ = h.pool.Exec(ctx,
				`INSERT INTO article_states (user_id, article_id, is_starred)
				 SELECT $1, a.id, false
				 FROM articles a JOIN sources s ON a.source_id = s.id
				 WHERE a.id = $2 AND s.user_id = $1 AND s.deleted_at IS NULL
				 ON CONFLICT (user_id, article_id) DO UPDATE SET is_starred = false`,
				user.ID, articleID,
			)
		}

	case "feed":
		feedID, err := strconv.ParseInt(id, 10, 64)
		if err != nil || as != "read" {
			return
		}
		beforeTS, _ := strconv.ParseInt(c.PostForm("before"), 10, 64)
		if beforeTS > 0 {
			before := time.Unix(beforeTS, 0)
			_, _ = h.pool.Exec(ctx,
				`INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
				 SELECT $1, a.id, true, now()
				 FROM articles a
				 JOIN sources s ON a.source_id = s.id
				 WHERE s.id = $2 AND s.user_id = $1 AND s.deleted_at IS NULL
				   AND a.published_at < $3
				 ON CONFLICT (user_id, article_id) DO UPDATE SET is_read = true, last_read_at = now()`,
				user.ID, feedID, before,
			)
		}

	case "group":
		if as != "read" {
			return
		}
		beforeTS, _ := strconv.ParseInt(c.PostForm("before"), 10, 64)
		if beforeTS > 0 {
			before := time.Unix(beforeTS, 0)
			_, _ = h.pool.Exec(ctx,
				`INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
				 SELECT $1, a.id, true, now()
				 FROM articles a
				 JOIN sources s ON a.source_id = s.id
				 WHERE s.user_id = $1 AND s.deleted_at IS NULL
				   AND a.published_at < $2
				 ON CONFLICT (user_id, article_id) DO UPDATE SET is_read = true, last_read_at = now()`,
				user.ID, before,
			)
		}
	}
}

// SetFeverPassword handles POST /api/users/me/fever.
// It computes the Fever API key from the user's GitHub username and a chosen password,
// stores the SHA-256 hash, and returns the raw key (shown once).
func (h *Handler) SetFeverPassword(c *gin.Context) {
	u := middleware.GetUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	type request struct {
		Password string `json:"password"`
	}
	var req request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 6 characters"})
		return
	}

	// Compute: apiKey = MD5(github_username:password)
	apiKey := ComputeFeverAPIKey(u.GitHubUsername, req.Password)

	// Hash for storage: hashedKey = SHA-256(apiKey)
	hashedKey := HashFeverKey(apiKey)

	// Store the hash
	_, err := h.pool.Exec(c.Request.Context(),
		"UPDATE users SET fever_api_key = $2 WHERE id = $1",
		u.ID, hashedKey,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save fever api key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_key":   apiKey,
		"fever_url": "/fever/",
		"username":  u.GitHubUsername,
	})
}
