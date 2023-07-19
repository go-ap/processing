# About GoActivityPub: Processing

[![MIT Licensed](https://img.shields.io/github/license/go-ap/processing.svg)](https://raw.githubusercontent.com/go-ap/processing/master/LICENSE)
[![Build Status](https://builds.sr.ht/~mariusor/processing.svg)](https://builds.sr.ht/~mariusor/processing)
[![Test Coverage](https://img.shields.io/codecov/c/github/go-ap/processing.svg)](https://codecov.io/gh/go-ap/processing)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-ap/processing)](https://goreportcard.com/report/github.com/go-ap/processing)
<!-- [![Codacy Badge](https://api.codacy.com/project/badge/Grade/29664f7ae6c643bca76700143e912cd3)](https://www.codacy.com/app/go-ap/processing/dashboard) -->

This project is part of the [GoActivityPub](https://github.com/go-ap) library which helps with creating ActivityPub applications using the Go programming language.

It provides basic functionality for processing and validation of generic ActivityPub activities.

It uses the concepts detailed in the [5.8.1 Content Management](https://www.w3.org/TR/activitystreams-vocabulary/#motivations)
section of the ActivityStreams vocabulary specification for logic separation between groups of activities.

For the actual processing it uses the [6. Client to Server Interactions](https://www.w3.org/TR/activitypub/#client-to-server-interactions)
and [7. Server to Server Interactions](https://www.w3.org/TR/activitypub/#server-to-server-interactions) sections of the ActivityPub specification.

You can find an expanded documentation about the whole library [on SourceHut](https://man.sr.ht/~mariusor/go-activitypub/go-ap/index.md).

For discussions about the projects you can write to the discussions mailing list: [~mariusor/go-activitypub-discuss@lists.sr.ht](mailto:~mariusor/go-activitypub-discuss@lists.sr.ht)

For patches and bug reports please use the dev mailing list: [~mariusor/go-activitypub-dev@lists.sr.ht](mailto:~mariusor/go-activitypub-dev@lists.sr.ht)
