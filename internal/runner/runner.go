package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"funpay-parser/internal/config"
	"funpay-parser/internal/duration"
	"funpay-parser/internal/llm"
	"funpay-parser/internal/models"
	"funpay-parser/internal/scraper"
)

type Options struct {
	CategoryID    int    `json:"category_id"`
	Query         string `json:"query"`
	UseSearch     bool   `json:"use_search"`
	Pages         int    `json:"pages"`
	MaxPages      int    `json:"max_pages"`
	Candidates    int    `json:"candidates"`
	Deep          bool   `json:"deep"`
	ScrapeWorkers int    `json:"scrape_workers"`
	LLMWorkers    int    `json:"llm_workers"`
	FunpayProxy   string `json:"funpay_proxy,omitempty"`
}
type Result struct {
	Success    bool             `json:"success"`
	Error      string           `json:"error,omitempty"`
	Summary    map[string]int   `json:"summary,omitempty"`
	Cheapest   *models.Listing  `json:"cheapest"`
	AllResults []models.Listing `json:"all_results,omitempty"`
	Listings   []models.Listing `json:"listings,omitempty"`
}

type Cancelled struct{}

func (Cancelled) Error() string { return "stopped by user" }

type Classifier interface {
	ClassifyMany(ctx context.Context, listings []models.Listing, workers int, progress func(string)) []models.Listing
}

func Run(ctx context.Context, cfg config.Config, opt Options, progress func(string)) (Result, error) {
	return RunWithClassifier(ctx, cfg, opt, progress, llm.New(cfg))
}

func RunWithClassifier(ctx context.Context, cfg config.Config, opt Options, progress func(string), classifier Classifier) (Result, error) {
	if opt.CategoryID == 0 {
		opt.CategoryID = 1355
	}
	if opt.Query == "" {
		opt.Query = "chatgpt plus"
	}
	pages := opt.Pages
	if pages == 0 {
		pages = opt.MaxPages
	}
	if pages == 0 {
		pages = cfg.MaxPages
	}
	if opt.ScrapeWorkers == 0 {
		opt.ScrapeWorkers = 4
	}
	if opt.LLMWorkers == 0 {
		opt.LLMWorkers = 8
	}
	if opt.FunpayProxy != "" {
		cfg.Proxy = opt.FunpayProxy
		cfg.ProxyURL = config.ParseProxy(opt.FunpayProxy)
	}
	target := duration.ExtractTargetDuration(opt.Query)
	s := scraper.New(cfg)
	var listings []models.Listing
	var err error
	if opt.UseSearch {
		progress(fmt.Sprintf("🔍 Searching Funpay for '%s'...", opt.Query))
		listings, err = s.Search(ctx, opt.Query, pages, opt.ScrapeWorkers)
	} else {
		progress(fmt.Sprintf("🔍 Fetching Funpay category (ID=%d) in parallel...", opt.CategoryID))
		listings, err = s.FetchCategory(ctx, opt.CategoryID, pages, true, opt.ScrapeWorkers)
	}
	if ctx.Err() != nil {
		return Result{}, Cancelled{}
	}
	if err != nil {
		progress("⚠️ Funpay warning: " + err.Error())
	}
	progress(fmt.Sprintf("📦 Found %d listings", len(listings)))
	if target != nil {
		progress(fmt.Sprintf("⏱️ Duration filter: %.0f days from query", *target))
	}
	if len(listings) == 0 {
		return Result{Success: false, Error: "No listings found", Listings: []models.Listing{}, Cheapest: nil}, nil
	}
	filtered := scraper.FilterShared(listings)
	progress(fmt.Sprintf("🧹 Filtered out %d explicitly shared listings, %d left for LLM", len(listings)-len(filtered), len(filtered)))
	if len(filtered) == 0 {
		return Result{Success: false, Error: "No listings left after filtering", Listings: []models.Listing{}, Cheapest: nil}, nil
	}
	filtered = selectCandidates(filtered, target, opt.Candidates, progress)
	if opt.Deep {
		progress("🌐 Fetching full descriptions in parallel...")
		filtered = fetchDescriptions(ctx, s, filtered, opt.ScrapeWorkers)
	}
	if ctx.Err() != nil {
		return Result{}, Cancelled{}
	}
	if classifier == nil {
		classifier = llm.New(cfg)
	}
	progress(fmt.Sprintf("🧠 Classifying listings with LLM (%d workers)...", opt.LLMWorkers))
	classified := classifier.ClassifyMany(ctx, filtered, opt.LLMWorkers, progress)
	if ctx.Err() != nil {
		return Result{}, Cancelled{}
	}
	all := merge(classified, listings, filtered)
	cheapest := FindCheapestPersonal(classified, target)
	personal, shared, other := 0, 0, 0
	for _, l := range classified {
		if l.IsPlus != nil && *l.IsPlus {
			at := ""
			if l.AccountType != nil {
				at = *l.AccountType
			}
			switch at {
			case "personal":
				personal++
			case "shared":
				shared++
			default:
				other++
			}
		}
	}
	if cheapest != nil {
		progress("✅ Cheapest personal account found!")
	} else {
		progress("❌ No personal account confirmed.")
	}
	b, _ := json.MarshalIndent(all, "", "  ")
	_ = os.WriteFile("results.json", b, 0644)
	progress("💾 Full results saved to results.json")
	return Result{Success: true, Summary: map[string]int{"total_plus": len(listings), "classified": len(classified), "personal": personal, "shared": shared, "other": other}, Cheapest: cheapest, AllResults: all}, nil
}

func selectCandidates(listings []models.Listing, target *float64, limit int, progress func(string)) []models.Listing {
	if len(listings) == 0 {
		return listings
	}

	product := make([]models.Listing, 0, len(listings))
	nonProduct := 0
	for _, l := range listings {
		if isClearlyNonProduct(l) {
			nonProduct++
			continue
		}
		product = append(product, l)
	}
	if len(product) > 0 {
		listings = product
		if nonProduct > 0 {
			progress(fmt.Sprintf("🧯 Dropped %d guide/prompt/request lots before LLM", nonProduct))
		}
	}

	if target != nil {
		exact := []models.Listing{}
		compatible := []models.Listing{}
		conflicting := 0
		for _, l := range listings {
			text := listingText(l)
			switch {
			case duration.Matches(text, target, false):
				exact = append(exact, l)
			case duration.Matches(text, target, true):
				compatible = append(compatible, l)
			default:
				conflicting++
			}
		}
		sortCandidates(exact)
		sortCandidates(compatible)
		prioritized := append(exact, compatible...)
		if len(prioritized) > 0 {
			listings = prioritized
			progress(fmt.Sprintf("⏱️ Candidate duration priority: %d exact, %d unknown/compatible, %d conflicting skipped", len(exact), len(compatible), conflicting))
		} else {
			progress("⚠️ No duration-compatible candidates found; falling back to cheapest filtered listings")
			sortCandidates(listings)
		}
	} else {
		sortCandidates(listings)
	}

	if limit > 0 && len(listings) > limit {
		listings = listings[:limit]
		progress(fmt.Sprintf("🎯 Limited LLM candidates to %d best filtered listings", len(listings)))
	} else {
		progress(fmt.Sprintf("🎯 Sending %d filtered listings to LLM", len(listings)))
	}
	return listings
}

func sortCandidates(listings []models.Listing) {
	sort.SliceStable(listings, func(i, j int) bool {
		si := candidateScore(listings[i])
		sj := candidateScore(listings[j])
		if si != sj {
			return si > sj
		}
		return listings[i].Price < listings[j].Price
	})
}

func candidateScore(l models.Listing) int {
	text := strings.ToLower(listingText(l))
	score := 0
	for _, k := range []string{"personal", "private", "личный", "приватный", "персональный", "individual", "single owner", "one owner"} {
		if strings.Contains(text, k) {
			score += 8
		}
	}
	for _, k := range []string{"shared", "общий", "common account", "common", "rental", "rent", "1 hour", "2 hour", "3 hour", "6 hour", "час"} {
		if strings.Contains(text, k) {
			score -= 6
		}
	}
	if l.Price <= 0 {
		score -= 10
	}
	return score
}

func isClearlyNonProduct(l models.Listing) bool {
	text := strings.ToLower(listingText(l))
	for _, k := range []string{"guide", "гайд", "руководство", "prompt", "prompts", "requests", "запрос", "smart purchase", "profitable purchase", "getting great deals"} {
		if strings.Contains(text, k) {
			return true
		}
	}
	return false
}

func listingText(l models.Listing) string {
	return l.Title + " " + l.Description
}

func fetchDescriptions(ctx context.Context, s *scraper.Client, in []models.Listing, workers int) []models.Listing {
	jobs := make(chan int)
	out := make(chan models.Listing, len(in))
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				l := in[i]
				l.Description = s.FetchDescription(ctx, l)
				out <- l
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := range in {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
			}
		}
	}()
	go func() { wg.Wait(); close(out) }()
	res := []models.Listing{}
	for l := range out {
		res = append(res, l)
	}
	return res
}
func merge(classified, original, filtered []models.Listing) []models.Listing {
	m := map[string]models.Listing{}
	for _, l := range original {
		m[l.ID] = l
	}
	for _, l := range classified {
		m[l.ID] = l
	}
	out := make([]models.Listing, 0, len(m))
	for _, l := range m {
		out = append(out, l)
	}
	return out
}
func FindCheapestPersonal(list []models.Listing, target *float64) *models.Listing {
	cands := []models.Listing{}
	for _, l := range list {
		if l.IsPlus != nil && *l.IsPlus && l.AccountType != nil && *l.AccountType == "personal" && l.Price > 0 {
			cands = append(cands, l)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	pick := func(xs []models.Listing) *models.Listing {
		sort.Slice(xs, func(i, j int) bool { return xs[i].Price < xs[j].Price })
		return &xs[0]
	}
	if target == nil {
		return pick(cands)
	}
	match := []models.Listing{}
	fallback := []models.Listing{}
	for _, l := range cands {
		text := l.Title + " " + l.Description
		if duration.Matches(text, target, false) {
			match = append(match, l)
		}
		if duration.Matches(text, target, true) {
			fallback = append(fallback, l)
		}
	}
	if len(match) > 0 {
		return pick(match)
	}
	if len(fallback) > 0 {
		return pick(fallback)
	}
	return nil
}
