# High Stile

A static site generator written in Go, aiming to be as minimal as possible. It
delegates the conversion of Markdown to HTML to [pandoc][], so you will need
that installed as well.

Features:

* Generates a blog
* Creates hard links to avoid excessive copies of static files
* Converts Markdown to HTML using [pandoc][]
* Pages and posts can be HTML or Markdown
* Generates an RSS feed and a [JSON feed][jsonfeed]

I don't recommend that you use this as I wrote it for my [personal
site][site].

## License

MIT.

[pandoc]: http://pandoc.org
[site]: http://www.thomasdenney.co.uk
[jsonfeed]: https://jsonfeed.org/
