// Package catalog holds the products that have been dispensed.
//
// A product is reference data with a stable identity that outlives any single
// prescription: the same cultivar from the same manufacturer is the same
// product in March and in November. How many grams of it are left is not stored
// here, it is a fold over the journal.
package catalog
