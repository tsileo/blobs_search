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

return {parse_tag = parse_tag}
