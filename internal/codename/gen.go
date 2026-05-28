// Package codename produces random adjective-animal pair names
// (e.g. "lucky-otter") for worktree directories and branches.
package codename

import (
	"math/rand"
	"strings"
)

var adjectives = []string{
	"happy", "lucky", "brave", "swift", "silent", "shiny", "bold", "calm",
	"eager", "fierce", "gentle", "jolly", "kind", "mighty", "noble", "proud",
	"quiet", "rapid", "sleek", "vivid", "wise", "breezy", "sunny", "stormy",
	"sturdy", "clever", "cosmic", "crimson", "dapper", "electric", "humble",
	"nimble", "plucky", "radiant", "tidy", "zesty",
}

var animals = []string{
	"otter", "tiger", "panda", "fox", "lark", "robin", "pelican", "wolf",
	"eagle", "lynx", "badger", "owl", "falcon", "hare", "raven", "dolphin",
	"jaguar", "koala", "mole", "narwhal", "octopus", "pony", "quail", "raccoon",
	"seal", "toad", "urchin", "viper", "walrus", "yak", "ferret", "gecko",
	"heron", "impala", "marmot", "newt", "orca",
}

// New returns a new "<adjective>-<animal>" codename.
func New() string {
	adj := adjectives[rand.Intn(len(adjectives))]
	an := animals[rand.Intn(len(animals))]
	return adj + "-" + an
}

// Sanitize replaces "/" with "-" so a branch name can safely be used as a path leaf.
func Sanitize(s string) string {
	return strings.ReplaceAll(s, "/", "-")
}
