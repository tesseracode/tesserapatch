package workflow

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const (
	rgaS3DocADR                 = "docs/adrs/ADR-036-recipe-coverage-authority.md"
	rgaS3DocPRD                 = "docs/prds/PRD-recipe-generation-authority.md"
	rgaS3DocADRHeading          = "### D3 - Add deterministic `artifacts/recipe-coverage.json` (canonical schema)"
	rgaS3DocPRDSchemaHeading    = "### 6.4 Coverage wire format"
	rgaS3DocPRDPredicateHeading = "### 6.5 Completeness and simulation"
	rgaS3DocSchemaSHA           = "729f841487e3a0fc6d3790d865ea83d8c90396c02342894502827335c42a6717"
	rgaS3DocPredicateSHA        = "6b2f2a80654056d16050f59b2f26702963188fe671c3ce0569adc5448ac5863e"
)

type rgaS3DocReference struct {
	numbers    string
	contextSHA string
}

// Accepted rev-7 bindings, captured at the S3 WAVE_BASE. Each fingerprint
// binds 80 bytes on each side of a reference to its number(s), not merely to
// the allowed numeric range. Historical errata quoting an old wrong number
// remain historical claims: their accepted contexts are checked too.
var rgaS3DocADRReferences = []rgaS3DocReference{
	{"1", "e828299eb9e692fa098609bbb2d229359c0509ea674a7603267c11e960e5232b"},
	{"8", "ea1532903aba7555263061bf2e5aa1e38be514360156ac3c6b2a971a98bb81f7"},
	{"10", "b544d96439c53fa50296eabc7a81b856934b0b140cabe99390ccd90df700ce10"},
	{"8", "cb836048c2cfc4974d0c43c6c96c3cfd4e5e6ca52c793fc6084b19fb0cf19911"},
	{"3", "e60aa599db45918e822f10d299d36604e16dc4d231c907f9d0cc4fcff5e11400"},
	{"8", "e159c2438ac17fde67fd0f999abbd2abbf6647451a4316ead2bea963726e09c1"},
	{"8", "218cbc823e6ec72b444cbef87634a68b4e43e75db91e5b40720773ed0de62c4c"},
	{"7", "6c3ace6ec63a11ea8338dbb84d7cd04163a0979407ede30698927158d422cb9a"},
	{"8", "01d42f243d45bfe701123573b6ceefe1bb84a7960f68fac8d4dd9ba1a24372bc"},
	{"8", "dd37a6671ca32f0d89028deace2af4a4e70d07d99a5962b18a72fba847ea66c6"},
	{"1", "555bde8fd95195c5f7169420f5b0997ae29202e3682ff9cd8d8c948b78590f12"},
	{"1", "682fbb3d78636ea450aa0aba8fec7d0771f2421b8ca5a79f3797beb3b9fd5eff"},
	{"3", "845f403f324bc620b575c74a2ff92e787d76c032e6c3d4e7741abfb3e2b09ed3"},
	{"1 and 3", "7131d974e6e76ae9256d522d51acb9733c0440cd4671b0258509f805b8a848ab"},
	{"2", "95752a94285c7d0b745266d6f120f6ebfad44e9e6538329178d78d2e19ef284e"},
	{"4-7", "6b2e4e645614b4da5caf8c31bac5ddbdc21baed6ec837d0c845ff66b30796bf1"},
	{"8-10", "61c09788301a5deabd842f36cb02b291518ecf1ab1d3bc6daef778d7047c0cf3"},
	{"1", "a8ad1da495fb565ea70e906a40096148afe171fcdb6e8f15445987688c447880"},
	{"8", "bc58069addacf36ebb858db5c769466500ae5bc7dfb9b8f46189500d1a9c42e8"},
	{"3", "01fbdc378909269e1d3102eecd8d1b5b2d8cfe0495a45dc36f9ce9a485b35e29"},
	{"3", "fbcaf0adcf3fd668cab32b1e648fd460abb828cbf9d64b7fe7e4961502d8a6b6"},
	{"3", "f2633fd329ff50fab77d37a1c8103b1ced6eff05cb6aed960b6666e1d2db1ee5"},
}

var rgaS3DocPRDReferences = []rgaS3DocReference{
	{"1", "e828299eb9e692fa098609bbb2d229359c0509ea674a7603267c11e960e5232b"},
	{"8", "cb9bad38fac7c54e1aaeeee9931f4bb9ad389dae03ba586fb62ff8f70189a7ca"},
	{"10", "fcc01277910eef8453cdaff44834e4fdd3825cbe1dc503eed0e43eb26988227a"},
	{"3", "963527ea7cc86843e0a5e3e861dc41622a126b4405290e1c38249c3879a398db"},
	{"1-4", "fc71827b1854db7f790064908d5a724a42f357a748db9e6d221aca375bcf4595"},
	{"1-3", "92f5da7bae4a78d8f47c131df6ea92613e5b655d7395e323df65c80d77325e70"},
	{"1", "2a50ba295afa7d086238266f79a5683d9522b760ae2b1f9e5fd66a4f70300f89"},
	{"1", "b6d7f9747d8f724b810adf6503323e254eef5253115a9943bf75a923a3614717"},
	{"8", "93dfde9e6f4dd027b87d6b6a44a204289872f93ae5ff886049b931653bc42a94"},
	{"10", "6e327706815aa44390669b805b1269f8845760b49d3167a4395ef933073b6994"},
	{"8", "e519e8c9e743f7f989dc2c244c3d6f617c1a556ba2bccce56dbbacba04781caa"},
	{"1 and 3", "3b40974a0baba26e031e37a1d5698de0ce1ad61a95fc77a45d6d7cfdfc71ea9c"},
	{"2", "6f7468f39faaebf33c6b57939f45a7a9c0663bd0e88fbdde6736de75df1e0239"},
	{"4-7", "3782bf90ea2880c0093d2165f34ae7cb292f55724810b60871491fd7ec28cc99"},
	{"8-10", "f3fdc6156c24e2d38616fcec67d8c0f7cba12b5588d24824093f0fbd7a26bc8b"},
	{"1", "f4abbd6b29f687d6cd7d9900a4bac2504d952ce29863292235737d95c520f848"},
	{"8", "d1881797493e7eedd42d5aaa4492310b4abdfc6562c787f5d0614ca757d6f3df"},
	{"10", "b96d707e264305270dbc1cca3aca5cabef094ba802ff164d12aa96fc3c157bf4"},
	{"8", "bf729b65d3208fbfeaa9273f6e1e19706fa4b8a568cc228032bf23b7b6a2d5d3"},
	{"9", "9521ed74f99b57a44fed3689dc6ef8f32be0432bbfc145a017f0c184a6e0bfcd"},
	{"7", "9bd357d46526a9a22dd682da84e975d3971811e5eeb41b50feea4a693cfbcd51"},
	{"10", "904c10df120222f2f7c00eac524219882310e85c8a5436579ff9c96de7770f05"},
	{"3", "136c94eaf6a198464f395803fd4ea74f41203dbb5df3c93194e616df217d1ad9"},
	{"3", "627671bdd48d74d19bf5a1504334a9512457fda2440f8618696cdf54fb2ff5f7"},
	{"3", "912c727e7d800cba25be9ea0690bc79a5cea57060466f42920e8348a6dc34b1c"},
	{"3", "90f6da566e25016a2e7366d7a60d2f049ee846c2434e9aae284e5ab949cec22c"},
	{"8", "eec5ef7d9a4c7fff32b42813040f2ce34280f4214a020d9d23d561dfe34e126b"},
	{"8", "c1e79b22e8aea5f55cd5b2624243f0e39cce4fcdbb43b358520fe25b398d6284"},
	{"8", "d2d7807661c24a1fe93c87724cbc828657218cd2579741efdbb0b75bf388d421"},
	{"7", "787ac2abb5300b776748beb9ab6466bbc3a5c91623ea43a132b346ed94e60a7d"},
	{"1", "e0688bdded16014ea51d5c7ce0b4eae1a4c74b826a2fb019683a154c94575b97"},
	{"7", "652c4a43c10032bf48c3258b2646856a8cd097a41fcb1e90689358d2f498669f"},
	{"3", "3f5de4786a760fc61813b71f952d27ff5ca2f545eaf5bd2f919d286d536d3975"},
	{"8", "8d4f59c8c55bffac7a28b9e5a822e1ea9f5cdae10714f8b4c9a725baae7e529d"},
	{"10", "d095ade9e54408e89526968cadeabe461f7fbde3fe7595318be3e95e40cfdaa3"},
	{"1-3", "dc5b2060147ca57fb4c27f1f7ef32a25e36047bf308ad95092d961e297a6ec65"},
	{"8", "6d2e1601cf401392d1cc8609645e10437d91e2bf906f9238903632c0a0688a82"},
	{"1", "3001e80b461298aaf34deadb2fe6f59ab37824b25579b64ee47f1bc02a866b50"},
	{"8", "8b488446d447201501495601de27622e3a3fa6b9dfa2ade921fd0c7fce8f1c5c"},
	{"10", "612e97c2961a8401ab5077de66d56622b29a328ec53f62f75c6ee3c80265a3ae"},
	{"3", "6ad535d5b3de8d1a4851e9b5d9af4fc7967b151506b1d177b341470067196949"},
	{"1-4", "a746a44a7901f4ef9fc577a68e6111992c0cb333986e31a544163dc25d28672a"},
	{"1-3", "7523c6e5d4744df81baeb800f40f760242a7f976aae0d93f5a30bfbcdd3963d0"},
	{"8", "7b5309ca9b90d574f895daff9727bff77ab459b0774873ca265eaf087a7e433e"},
}

func rgaS3DocHash(text string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
}

func rgaS3DocSection(doc, heading string) (string, error) {
	needle := heading + "\n"
	start := strings.Index(doc, needle)
	if start < 0 || (start > 0 && doc[start-1] != '\n') || strings.Count(doc, needle) != 1 {
		return "", fmt.Errorf("canonical heading missing or ambiguous: %s", heading)
	}
	level := strings.IndexByte(heading, ' ')
	offset := start + len(needle)
	for _, line := range strings.SplitAfter(doc[offset:], "\n") {
		n := 0
		for n < len(line) && line[n] == '#' {
			n++
		}
		if n > 0 && n <= level && n < len(line) && line[n] == ' ' {
			return doc[start:offset], nil
		}
		offset += len(line)
	}
	return doc[start:], nil
}

func rgaS3DocSchema(doc, heading string) (string, error) {
	section, err := rgaS3DocSection(doc, heading)
	if err != nil {
		return "", err
	}
	const fence = "```json\n"
	if strings.Count(section, fence) != 1 {
		return "", fmt.Errorf("canonical schema fence missing or ambiguous")
	}
	_, body, _ := strings.Cut(section, fence)
	end := strings.Index(body, "\n```")
	if end < 0 {
		return "", fmt.Errorf("canonical schema fence not closed")
	}
	return body[:end+1], nil
}

func rgaS3DocPredicates(doc, heading string) (string, error) {
	section, err := rgaS3DocSection(doc, heading)
	if err != nil {
		return "", err
	}
	const first = "  1. `patch_present` is `true`,"
	const last = "      already-present and writes no byte.\n"
	start := strings.Index(section, first)
	end := strings.Index(section, last)
	if start < 0 || end < start || strings.Count(section, first) != 1 || strings.Count(section, last) != 1 {
		return "", fmt.Errorf("canonical predicate block missing or ambiguous")
	}
	block := section[start : end+len(last)]
	numbers := regexp.MustCompile(`(?m)^  ([0-9]+)\. `).FindAllStringSubmatch(block, -1)
	if len(numbers) != 10 {
		return "", fmt.Errorf("predicate count = %d, want 10", len(numbers))
	}
	for i, number := range numbers {
		if number[1] != strconv.Itoa(i+1) {
			return "", fmt.Errorf("predicate ordinal %d is %q", i+1, number[1])
		}
	}
	return block, nil
}

func rgaS3DocValidateBlocks(adr, prd string) error {
	for _, block := range []struct {
		name, adrHeading, prdHeading, expectedSHA string
		extract                                   func(string, string) (string, error)
	}{
		{"schema", rgaS3DocADRHeading, rgaS3DocPRDSchemaHeading, rgaS3DocSchemaSHA, rgaS3DocSchema},
		{"predicates", rgaS3DocADRHeading, rgaS3DocPRDPredicateHeading, rgaS3DocPredicateSHA, rgaS3DocPredicates},
	} {
		left, err := block.extract(adr, block.adrHeading)
		if err != nil {
			return fmt.Errorf("ADR %s: %w", block.name, err)
		}
		right, err := block.extract(prd, block.prdHeading)
		if err != nil {
			return fmt.Errorf("PRD %s: %w", block.name, err)
		}
		if left != right {
			return fmt.Errorf("%s blocks are not byte-identical", block.name)
		}
		if rgaS3DocHash(left) != block.expectedSHA {
			return fmt.Errorf("%s blocks changed from the accepted rev-7 contract", block.name)
		}
	}
	return nil
}

func rgaS3DocReferences(doc string) []rgaS3DocReference {
	text := strings.Join(strings.Fields(strings.NewReplacer("`", "", "*", "").Replace(doc)), " ")
	matches := regexp.MustCompile(`(?i)\bpredicates?\s+(\d+(?:\s*(?:-|\x{2013}|and|,)\s*\d+)*)`).FindAllStringSubmatchIndex(text, -1)
	out := make([]rgaS3DocReference, 0, len(matches))
	for _, m := range matches {
		context := text[max(0, m[0]-80):m[0]] + "predicate <ref>" + text[m[1]:min(len(text), m[1]+80)]
		out = append(out, rgaS3DocReference{text[m[2]:m[3]], rgaS3DocHash(context)})
	}
	return out
}

func rgaS3DocValidateReferences(doc string, expected []rgaS3DocReference) error {
	actual := rgaS3DocReferences(doc)
	if len(actual) != len(expected) {
		return fmt.Errorf("predicate reference inventory = %d, want %d", len(actual), len(expected))
	}
	for i := range expected {
		if actual[i] != expected[i] {
			return fmt.Errorf("predicate reference %d changed context or target: got %+v, want %+v", i+1, actual[i], expected[i])
		}
	}
	return nil
}

func TestRGAS3DocCanonicalBlockParity(t *testing.T) {
	adr, prd := rgaS0ReadRepoFile(t, rgaS3DocADR), rgaS0ReadRepoFile(t, rgaS3DocPRD)
	if err := rgaS3DocValidateBlocks(adr, prd); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, old, replacement string
		both                   bool
	}{
		{"prd-schema-field-drift", `"recipe_decodable": true`, `"recipe_decodable": false`, false},
		{"both-schemas-drift", `"schema_version": 1`, `"schema_version": 2`, true},
		{"prd-predicate-list-reworded", "so the reference is durable;", "so the reference is optional;", false},
		{"both-predicate-lists-loosened", "exact postimage byte, existence and mode set", "postimage path set", true},
		{"predicate-dropped", "  2. `reference.kind` is `commit`, so the reference is durable;\n", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(prd, tc.old) || (tc.both && !strings.Contains(adr, tc.old)) {
				t.Fatalf("mutation anchor missing: %q", tc.old)
			}
			wrongPRD := strings.Replace(prd, tc.old, tc.replacement, 1)
			wrongADR := adr
			if tc.both {
				wrongADR = strings.Replace(adr, tc.old, tc.replacement, 1)
			}
			if err := rgaS3DocValidateBlocks(wrongADR, wrongPRD); err == nil {
				t.Fatal("same block validator accepted a changed contract")
			}
		})
	}
	if err := rgaS3DocValidateBlocks(adr+"\n"+rgaS3DocADRHeading+"\n", prd); err == nil {
		t.Fatal("ambiguous canonical heading accepted")
	}
}

func TestRGAS3DocAllPredicateReferences(t *testing.T) {
	adr, prd := rgaS0ReadRepoFile(t, rgaS3DocADR), rgaS0ReadRepoFile(t, rgaS3DocPRD)
	for _, doc := range []struct {
		name, text string
		refs       []rgaS3DocReference
	}{
		{"ADR", adr, rgaS3DocADRReferences}, {"PRD", prd, rgaS3DocPRDReferences},
	} {
		if err := rgaS3DocValidateReferences(doc.text, doc.refs); err != nil {
			t.Fatalf("%s: %v", doc.name, err)
		}
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"predicate-number-points-at-wrong-predicate", "present state (§6.5 predicate 10)", "present state (§6.5 predicate 8)"},
		{"uppercase-reference-retargeted", "Predicate 9 is scoped deliberately.", "Predicate 8 is scoped deliberately."},
		{"reference-removed", "present state (§6.5 predicate 10)", "present state (§6.5)"},
		{"out-of-range-reference", "present state (§6.5 predicate 10)", "present state (§6.5 predicate 11)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(prd, tc.old) {
				t.Fatalf("reference mutation anchor missing: %q", tc.old)
			}
			wrong := strings.Replace(prd, tc.old, tc.replacement, 1)
			if err := rgaS3DocValidateBlocks(adr, wrong); err != nil {
				t.Fatalf("reference fixture unexpectedly changed a canonical block: %v", err)
			}
			if err := rgaS3DocValidateReferences(wrong, rgaS3DocPRDReferences); err == nil {
				t.Fatal("same reference validator accepted a retargeted/removed reference")
			}
		})
	}
	for _, extra := range []string{"\nAdded authority: predicate 10.\n", "\nAdded authority: **predicate** **8**.\n"} {
		if err := rgaS3DocValidateReferences(prd+extra, rgaS3DocPRDReferences); err == nil {
			t.Fatal("reference outside the registered inventory was missed")
		}
	}
}

func TestRGAS3DocReferenceScannerSensitivity(t *testing.T) {
	const prefix, suffix = "Reclassification requires ", " and no execution."
	expected := []rgaS3DocReference{{"10", rgaS3DocHash(prefix + "predicate <ref>" + suffix)}}
	for _, reference := range []string{"predicate 10", "Predicate 10", "`predicate` **10**", "predicate\n  10"} {
		if err := rgaS3DocValidateReferences(prefix+reference+suffix, expected); err != nil {
			t.Fatalf("supported reference shape %q rejected: %v", reference, err)
		}
	}
	for _, reference := range []string{"predicate 8", "predicate 11", "property 10"} {
		if err := rgaS3DocValidateReferences(prefix+reference+suffix, expected); err == nil {
			t.Fatalf("same scanner/validator accepted wrong input %q", reference)
		}
	}
}
