local tknzr = require2('github.com/tsileo/blobstash_docstore_textsearch/tokenizer')
local qry = require2('github.com/tsileo/blobstash_docstore_textsearch/query')
local parse_tag = require('tokenizer').parse_tag

local fields = {'content', 'title'}
local tokenizer = tknzr:new()
tokenizer:add_parser(parse_tag)
local terms = tokenizer:parse(query.qs)

local q = qry:new(terms, fields)

match_tag = function(term, doc)
  local tags_index = {}
  if doc['tags'] ~= nil then
    for _,tag in ipairs(doc['tags']) do
      tags_index[tag] = true
    end
  end

  -- The term contains a tag
  if term.kind == 'tag' then

    if term.tag == 'tag' then
      -- check if the tag (as in tagging, the "query tag" value) is in the index
      if tags_index[term.value] == true then
        return true
      end
    end

    if term.tag == 'kind' then
      if doc['kind'] == term.value then
        return true
      end
    end

    if term.tag == 'created' or term.tag == 'updated' then
      date = doc[term.tag]
      -- If there's no updated field, set created instead
      if term.tag == 'updated' and doc['updated'] == nil then
        date = doc['created']
      end

      -- Check if there's a ">" or "<" modifier
      value = term.value
      tag_prefix = term.value:sub(1, 1)
      if tag_prefix == '>' or tag_prefix == '<' then
        value = value:sub(2, value:len())
      else
        tag_prefix = ''
      end

      if tag_prefix == '' and value == date:sub(1, value:len()) then
        return true
      elseif tag_prefix == '>' and date > value then
        return true
      elseif tag_prefix == '<' and date < value then
        return true
      end
    end

end
  return false
end

q:add_kind('tag', match_tag)

function match(doc)
  q:build_text_index(doc)
  return q:match(doc)
end

return match
