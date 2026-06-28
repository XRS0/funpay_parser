package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/proxy"

	"funpay-parser/internal/config"
	"funpay-parser/internal/models"
)

type Client struct {
	cfg  config.Config
	http *http.Client
}

func New(cfg config.Config) *Client {
	transport := &http.Transport{}
	if cfg.ProxyURL != "" {
		if u, err := url.Parse(cfg.ProxyURL); err == nil {
			d, err := proxy.FromURL(u, proxy.Direct)
			if err == nil {
				transport.Dial = d.Dial
			}
		}
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 30 * time.Second, Transport: transport}}
}

func (c *Client) CategoryURL(categoryID, page int) string {
	lang := ""
	if c.cfg.FunpayLang != "" && c.cfg.FunpayLang != "ru" {
		lang = c.cfg.FunpayLang + "/"
	}
	base := fmt.Sprintf("%s/%slots/%d/", c.cfg.FunpayBaseURL, lang, categoryID)
	if page > 1 {
		return base + "?page=" + strconv.Itoa(page)
	}
	return base
}
func (c *Client) SearchURL(query string, page int) string {
	lang := ""
	if c.cfg.FunpayLang != "" && c.cfg.FunpayLang != "ru" {
		lang = c.cfg.FunpayLang + "/"
	}
	v := url.Values{"query": []string{query}}
	if page > 1 {
		v.Set("page", strconv.Itoa(page))
	}
	return fmt.Sprintf("%s/%slots?%s", c.cfg.FunpayBaseURL, lang, v.Encode())
}

func (c *Client) getDoc(ctx context.Context, rawurl string) (*goquery.Document, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawurl, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125 Safari/537.36")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9,ru;q=0.8")
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
		} else {
			if resp.StatusCode < 400 {
				doc, parseErr := goquery.NewDocumentFromReader(resp.Body)
				_ = resp.Body.Close()
				return doc, parseErr
			}
			lastErr = fmt.Errorf("http %d", resp.StatusCode)
			_ = resp.Body.Close()
		}
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}
	}
	return nil, lastErr
}

func (c *Client) FetchCategory(ctx context.Context, categoryID int, maxPages int, onlyPlus bool, concurrency int) ([]models.Listing, error) {
	if maxPages <= 0 {
		maxPages = c.cfg.MaxPages
	}
	if concurrency <= 0 {
		concurrency = 4
	}
	return c.fetchPages(ctx, maxPages, concurrency, func(p int) string { return c.CategoryURL(categoryID, p) }, onlyPlus)
}
func (c *Client) Search(ctx context.Context, query string, maxPages int, concurrency int) ([]models.Listing, error) {
	if maxPages <= 0 {
		maxPages = c.cfg.MaxPages
	}
	res, err := c.fetchPages(ctx, maxPages, concurrency, func(p int) string { return c.SearchURL(query, p) }, false)
	if err != nil || len(res) == 0 {
		return c.FetchCategory(ctx, 1355, maxPages, true, concurrency)
	}
	return res, nil
}

func (c *Client) fetchPages(ctx context.Context, maxPages, concurrency int, urlFn func(int) string, onlyPlus bool) ([]models.Listing, error) {
	type pageRes struct {
		items []models.Listing
		err   error
	}
	jobs := make(chan int)
	out := make(chan pageRes, maxPages)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				items, err := c.fetchPage(ctx, urlFn(p), onlyPlus)
				out <- pageRes{items, err}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for p := 1; p <= maxPages; p++ {
			select {
			case <-ctx.Done():
				return
			case jobs <- p:
			}
		}
	}()
	go func() { wg.Wait(); close(out) }()
	seen := map[string]bool{}
	var res []models.Listing
	var firstErr error
	for r := range out {
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		for _, it := range r.items {
			if !seen[it.ID] {
				seen[it.ID] = true
				res = append(res, it)
			}
		}
	}
	return res, firstErr
}

func (c *Client) fetchPage(ctx context.Context, u string, onlyPlus bool) ([]models.Listing, error) {
	doc, err := c.getDoc(ctx, u)
	if err != nil {
		return nil, err
	}
	items := []models.Listing{}
	doc.Find("a.tc-item").Each(func(_ int, s *goquery.Selection) {
		if onlyPlus && !isPlusHint(s) {
			return
		}
		if l, ok := parseCard(c.cfg.FunpayBaseURL, s); ok {
			items = append(items, l)
		}
	})
	return items, nil
}

func (c *Client) FetchDescription(ctx context.Context, l models.Listing) string {
	doc, err := c.getDoc(ctx, l.URL)
	if err != nil {
		return l.Description
	}
	for _, sel := range []string{".tc-desc-text", ".lot-desc", ".description", "[class*='desc']"} {
		if t := strings.TrimSpace(doc.Find(sel).First().Text()); t != "" {
			return t
		}
	}
	return l.Description
}

var idRe = regexp.MustCompile(`[?&]id=(\d+)`)

func parseCard(base string, s *goquery.Selection) (models.Listing, bool) {
	href, _ := s.Attr("href")
	u, _ := url.Parse(href)
	b, _ := url.Parse(base)
	full := b.ResolveReference(u).String()
	id := "unknown"
	if m := idRe.FindStringSubmatch(href); len(m) > 1 {
		id = m[1]
	} else {
		id = regexp.MustCompile(`\D`).ReplaceAllString(href, "")
		if len(id) > 20 {
			id = id[:20]
		}
		if id == "" {
			id = "unknown"
		}
	}
	desc := strings.TrimSpace(s.Find(".tc-desc").Text())
	if desc == "" || full == "" {
		return models.Listing{}, false
	}
	price, cur := extractPrice(s.Find(".tc-price").First().Text())
	seller := strings.TrimSpace(s.Find(".tc-user").Text())
	html, _ := goquery.OuterHtml(s)
	return models.Listing{ID: id, Title: desc, Description: desc, Price: price, Currency: cur, Seller: seller, URL: full, RawHTML: html, ClassificationReason: fmt.Sprintf("hint: subscription=%s, type=%s", attr(s, "data-f-subscription"), attr(s, "data-f-type"))}, true
}
func attr(s *goquery.Selection, k string) string { v, _ := s.Attr(k); return v }
func isPlusHint(s *goquery.Selection) bool {
	return attr(s, "data-f-type") == "plus" || attr(s, "data-f-subscription") == "с подпиской"
}
func extractPrice(t string) (float64, string) {
	t = strings.ReplaceAll(strings.ReplaceAll(t, "\u00a0", ""), " ", "")
	t = strings.ReplaceAll(t, ",", ".")
	re := regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)`)
	m := re.FindStringSubmatch(t)
	if len(m) < 2 {
		return 0, ""
	}
	p, _ := strconv.ParseFloat(m[1], 64)
	cur := strings.TrimSpace(regexp.MustCompile(`[0-9\s.]`).ReplaceAllString(t, ""))
	return p, cur
}

var personal = []string{"личный", "приватный", "индивидуальный", "персональный", "не общий", "one owner", "personal", "private", "not shared", "single", "solo"}
var shared = []string{"общий", "shared", "general account", "общий доступ", "multiple", "several", "split", "group", "людей", "people"}

func FilterShared(list []models.Listing) []models.Listing {
	out := []models.Listing{}
	for _, l := range list {
		if score(l.Description) >= -2 {
			out = append(out, l)
		}
	}
	return out
}
func score(t string) int {
	t = strings.ToLower(t)
	n := 0
	for _, k := range personal {
		if strings.Contains(t, k) {
			n += 2
		}
	}
	for _, k := range shared {
		if strings.Contains(t, k) {
			n -= 2
		}
	}
	return n
}
