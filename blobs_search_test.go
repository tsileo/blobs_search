package main

import (
	"bytes"
	"reflect"
	"testing"

	luautil "a4.io/blobstash/pkg/apps/luautil"
	"a4.io/gluarequire2"

	"github.com/blevesearch/segment"
	"github.com/reiver/go-porterstemmer"
	"github.com/yuin/gopher-lua"
)

// copy-pasted from BlobStash
func stem(L *lua.LState) int {
	in := L.ToString(1)
	L.Push(lua.LString(porterstemmer.StemString(in)))
	return 1
}

// copy-pasted from BlobStash
func ltokenize(L *lua.LState) int {
	in := L.ToString(1)
	out, err := tokenize([]byte(in))
	if err != nil {
		panic(err)
	}
	L.Push(luautil.InterfaceToLValue(L, out))
	return 1
}

// copy-pasted from BlobStash
func tokenize(data []byte) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	segmenter := segment.NewWordSegmenter(bytes.NewReader(data))
	for segmenter.Segment() {
		if segmenter.Type() == segment.Letter {
			out[porterstemmer.StemString(segmenter.Text())] = true
		}
	}
	if err := segmenter.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type docMatch struct {
	doc   map[string]interface{}
	match bool
}

var preQuery = `
local tags = {tag = true, kind = true, updated = true, created = true}

function parse_tag(prefix, value)
  -- check if the term is quoted
  quoted = string.sub(value, 1, 1) == '"'

  -- check if term contains a colon (like 'tag:work')
  _, colon_count = value:gsub(':', '')
  contains_colon = colon_count == 1
  tag = ''
  tag_value = ''
  -- extract the tag (and it's value) if it looks there's one
  if not quoted and contains_colon then
    maybe_tag = string.sub(value, 1, string.find(value, ':')-1)
    if tags[maybe_tag] == true then
      tag = maybe_tag
      tag_value = string.sub(value, string.find(value, ':')+1, value:len())
      term = {value=tag_value, prefix=prefix, kind='tag', tag=tag}
      return term
    end
  end

  return nil
end
local t = require2('github.com/tsileo/blobstash_docstore_textsearch/tokenizer'):new()
t:add_parser(parse_tags)
return {terms = t:parse(query.qs)}
`

func TestBlobsSearchTokenizer(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	gluarequire2.NewRequire2Module(gluarequire2.NewRequireFromGitHub(nil)).SetGlobal(L)

	L.SetGlobal("debug", lua.LTrue)
	L.SetGlobal("porterstemmer", L.NewFunction(ltokenize))
	L.SetGlobal("porterstemmer_stem", L.NewFunction(stem))

	prepQuery := func(qs string) lua.LValue {
		L.SetGlobal("query", luautil.InterfaceToLValue(L, map[string]interface{}{
			"qs": qs,
		}))
		if err := L.DoString(preQuery); err != nil {
			panic(err)
		}
		ret := L.Get(-1)
		// t.Logf("%v", luautil.TableToMap(ret.(*lua.LTable)))
		return ret
	}
	for _, tdata := range []struct {
		qs       string
		expected []map[string]interface{}
	}{
		{
			"ok",
			[]map[string]interface{}{
				map[string]interface{}{"value": "ok", "kind": "text_stems", "prefix": ""},
			},
		},
		{
			"lol ok",
			[]map[string]interface{}{
				map[string]interface{}{"value": "lol", "kind": "text_stems", "prefix": ""},
				map[string]interface{}{"value": "ok", "kind": "text_stems", "prefix": ""},
			},
		},
		{
			"+lol ok",
			[]map[string]interface{}{
				map[string]interface{}{"value": "lol", "kind": "text_stems", "prefix": "+"},
				map[string]interface{}{"value": "ok", "kind": "text_stems", "prefix": ""},
			},
		},
		{
			"+lol -ok",
			[]map[string]interface{}{
				map[string]interface{}{"value": "lol", "kind": "text_stems", "prefix": "+"},
				map[string]interface{}{"value": "ok", "kind": "text_stems", "prefix": "-"},
			},
		},
		{
			"\"lol\" nope",
			[]map[string]interface{}{
				map[string]interface{}{"value": "lol", "kind": "text_match", "prefix": ""},
				map[string]interface{}{"value": "nope", "kind": "text_stems", "prefix": ""},
			},
		},
		{
			"\"lol\" +ok \"yes\" -no tag:boys +tag:work boys",
			[]map[string]interface{}{
				map[string]interface{}{"value": "lol", "kind": "text_match", "prefix": ""},
				map[string]interface{}{"value": "ok", "kind": "text_stems", "prefix": "+"},
				map[string]interface{}{"value": "yes", "kind": "text_match", "prefix": ""},
				map[string]interface{}{"value": "no", "kind": "text_stems", "prefix": "-"},
				map[string]interface{}{"value": "boys", "kind": "tag", "tag": "tag", "prefix": ""},
				map[string]interface{}{"value": "work", "kind": "tag", "tag": "tag", "prefix": "+"},
				map[string]interface{}{"value": "boi", "kind": "text_stems", "prefix": ""},
			},
		},
	} {
		out := prepQuery(tdata.qs)
		m := luautil.TableToMap(out.(*lua.LTable))
		// t.Logf("m=%+v\n", m)
		terms := []map[string]interface{}{}
		for _, term := range m["terms"].([]interface{}) {
			terms = append(terms, term.(map[string]interface{}))
		}
		if !reflect.DeepEqual(terms, tdata.expected) {
			t.Errorf("failed to split \"%s\", got %+v, expected %+v", tdata.qs, terms, tdata.expected)
		}
	}
}

func TestBlobsSearch(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	gluarequire2.NewRequire2Module(gluarequire2.NewRequireFromGitHub(nil)).SetGlobal(L)

	L.SetGlobal("debug", lua.LTrue)
	L.SetGlobal("porterstemmer", L.NewFunction(ltokenize))
	L.SetGlobal("porterstemmer_stem", L.NewFunction(stem))

	prepQuery := func(qs string) lua.LValue {
		L.SetGlobal("query", luautil.InterfaceToLValue(L, map[string]interface{}{
			"qs": qs,
		}))
		if err := L.DoFile("main.lua"); err != nil {
			panic(err)
		}
		ret := L.Get(-1)
		// t.Logf("%v", luautil.TableToMap(ret.(*lua.LTable)))
		return ret
	}

	matchDoc := func(q lua.LValue, doc map[string]interface{}) bool {
		if err := L.CallByParam(lua.P{
			Fn:      lua.LValue(q.(*lua.LFunction)),
			NRet:    1,
			Protect: true,
		}, luautil.InterfaceToLValue(L, doc)); err != nil {
			panic(err)
		}
		if L.Get(-1) == lua.LTrue {
			return true
		}
		return false
	}

	for _, tdata := range []struct {
		qs   string
		docs []docMatch
	}{
		{"ok", []docMatch{
			{map[string]interface{}{}, false},
			{map[string]interface{}{"content": "ok"}, true},
			{map[string]interface{}{"content": "lol", "title": "Ok it works"}, true},
			{map[string]interface{}{"content": "lol"}, false},
		}},
		{"penny", []docMatch{
			{map[string]interface{}{"content": "my two pennies"}, true},
			{map[string]interface{}{"content": "lol"}, false},
		}},
		{"lol ok", []docMatch{
			{map[string]interface{}{"content": "ok"}, true},
			{map[string]interface{}{"content": "lol"}, true},
			{map[string]interface{}{"title": "lol"}, true},
			{map[string]interface{}{"content": "lol", "title": "Ok it works"}, true},
		}},
		{"ok -lol", []docMatch{
			{map[string]interface{}{"content": "ok lol"}, false},
			{map[string]interface{}{"content": "lol ok"}, false},
			{map[string]interface{}{"content": "lol", "title": "Ok it works"}, false},
			{map[string]interface{}{"content": "ok"}, true},
		}},
		{"ok +lol", []docMatch{
			{map[string]interface{}{"content": "ok"}, false},
			{map[string]interface{}{"content": "lol"}, true},
			{map[string]interface{}{"content": "lol", "title": "Ok it works"}, true},
		}},
		{"\"ex act ma tch\"", []docMatch{
			{map[string]interface{}{"content": "exact match"}, false},
			{map[string]interface{}{"content": "lol lex act ma tch"}, true},
		}},
		{"+lol ok tag:work", []docMatch{
			{map[string]interface{}{"content": "lol"}, true},
			{map[string]interface{}{"content": "ok"}, false},
			{map[string]interface{}{"content": "lol", "title": "Ok it works"}, true},
		}},
		{"tag:work", []docMatch{
			{map[string]interface{}{"content": "lol", "tags": []interface{}{"work", "lol"}}, true},
			{map[string]interface{}{"content": "lol", "tags": []interface{}{"perso"}}, false},
		}},
		{"kind:note", []docMatch{
			{map[string]interface{}{"content": "lol", "tags": []interface{}{"work", "lol"}, "kind": "note"}, true},
			{map[string]interface{}{"content": "lol", "tags": []interface{}{"work", "lol"}, "kind": "file"}, false},
			{map[string]interface{}{"content": "lol", "tags": []interface{}{"perso"}}, false},
		}},
		{"created:2017 lol", []docMatch{
			{map[string]interface{}{"content": "lol", "tags": []interface{}{"work", "lol"}, "created": "2017-05-12T12:13:12"}, true},
			{map[string]interface{}{"content": "lol", "tags": []interface{}{"perso"}, "created": "2016-01-01T05:02:01Z"}, true},
			{map[string]interface{}{"content": "no", "tags": []interface{}{"perso"}, "created": "2016-03-05T04:21:12Z"}, false},
		}},
		{"updated:>2017-01-15", []docMatch{
			{map[string]interface{}{"content": "lol", "tags": []interface{}{"work", "lol"}, "created": "2017-02-02T04:32:12"}, true},
			{map[string]interface{}{"content": "nope", "tags": []interface{}{"perso"}, "created": "2016-01-01T01:01:01", "updated": "2017-01-02T05:01:02"}, false},
			{map[string]interface{}{"content": "no", "tags": []interface{}{"perso"}, "created": "2016-01-01T04:03:02"}, false},
		}},
		{"-created:2016 lol", []docMatch{
			{map[string]interface{}{"content": "lol", "tags": []interface{}{"work", "lol"}, "created": "2017-02-05T12:30:00"}, true},
			{map[string]interface{}{"content": "lol", "tags": []interface{}{"perso"}, "created": "2016-02-02T01:01:01"}, false},
			{map[string]interface{}{"content": "no", "tags": []interface{}{"perso"}, "created": "2016-05-06T02:02:02"}, false},
		}},
	} {
		q := prepQuery(tdata.qs)
		// t.Logf("q=%+v", q)
		// t.Logf("%v", luautil.TableToMap(q.(*lua.LTable)))
		for _, dmatch := range tdata.docs {
			match := matchDoc(q, dmatch.doc)
			t.Logf("query=\"%s\" doc=%+v, expected=%v, got=%v\n", tdata.qs, dmatch.doc, dmatch.match, match)
			if match != dmatch.match {
				t.Errorf("doc %+v test failed for query \"%s\", got %v, expected %v", dmatch.doc, tdata.qs, match, dmatch.match)
			}
		}
	}
}
