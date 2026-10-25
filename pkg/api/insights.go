// Package api serves a repository over HTTP; this file gives the costs their
// breakdowns: every priced purchase, and the purchases by strain, producer,
// pharmacy and year, for the charts and the drill-downs.
package api

import (
	"time"

	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// insights is prices.Insights as a client reads it, cents as cents.
type insights struct {
	Bought  []boughtOf `json:"bought"`
	Strains []groupOf  `json:"strains"`
	Makers  []groupOf  `json:"makers"`
	Sellers []groupOf  `json:"sellers"`
	Years   []groupOf  `json:"years"`
}

type boughtOf struct {
	Hash       string    `json:"hash"`
	At         time.Time `json:"at"`
	Fill       int       `json:"fill"`
	Product    string    `json:"product"`
	Name       string    `json:"name"`
	Maker      string    `json:"maker"`
	Strain     string    `json:"strain"`
	StrainName string    `json:"strain_name"`
	THC        float64   `json:"thc"`
	CBD        float64   `json:"cbd"`
	Grams      float64   `json:"grams"`
	Cents      int64     `json:"cents"`
	PerGram    float64   `json:"per_gram"`
	Seller     string    `json:"seller"`
}

type groupOf struct {
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Products  []string  `json:"products"`
	Purchases int       `json:"purchases"`
	Grams     float64   `json:"grams"`
	Cents     int64     `json:"cents"`
	PerGram   float64   `json:"per_gram"`
	Min       float64   `json:"min"`
	Max       float64   `json:"max"`
	First     float64   `json:"first"`
	Last      float64   `json:"last"`
	FirstAt   time.Time `json:"first_at"`
	LastAt    time.Time `json:"last_at"`
}

// insightsOf breaks down the purchases with a price, each product as the
// catalog has it.
func insightsOf(ws *workspace.Workspace) insights {
	product := func(slug string) prices.Product {
		if p, err := ws.Products.Find(slug); err == nil && p != nil {
			return prices.Product{Name: p.Name, Maker: p.Manufacturer, Cultivar: p.Cultivar, THC: p.THC, CBD: p.CBD}
		}
		return prices.Product{Name: slug}
	}
	in := prices.Insight(ws.State, prices.Latest(ws.Prices), product)
	out := insights{Bought: make([]boughtOf, 0, len(in.Bought)), Strains: groupsOf(in.Strains),
		Makers: groupsOf(in.Makers), Sellers: groupsOf(in.Sellers), Years: groupsOf(in.Years)}
	for _, b := range in.Bought {
		out.Bought = append(out.Bought, boughtOf{Hash: b.Hash, At: b.At, Fill: b.Fill + 1, Product: b.Product,
			Name: b.Name, Maker: b.Maker, Strain: b.Strain, StrainName: b.StrainName, THC: b.THC, CBD: b.CBD,
			Grams: b.Grams, Cents: b.Cents, PerGram: b.PerGram(), Seller: b.Seller})
	}
	return out
}

func groupsOf(in []prices.Group) []groupOf {
	out := make([]groupOf, 0, len(in))
	for _, g := range in {
		out = append(out, groupOf{Key: g.Key, Name: g.Name, Products: g.Products, Purchases: g.Purchases,
			Grams: g.Grams, Cents: g.Cents, PerGram: g.PerGram(), Min: g.Min, Max: g.Max, First: g.First,
			Last: g.Last, FirstAt: g.FirstAt, LastAt: g.LastAt})
	}
	return out
}
