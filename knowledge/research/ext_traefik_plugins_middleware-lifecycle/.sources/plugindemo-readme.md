---
url: https://github.com/traefik/plugindemo/blob/44419f66fe21c51f4c94fd46f8e02b98e4fb3168/readme.md
title: Developing a Traefik plugin
fetched: 2026-08-28
authority: official
---

A Traefik middleware plugin is a Go package that provides an `http.Handler`. Yaegi interprets it; it is not pre-compiled.

Plugins must be declared in static configuration. They are parsed and loaded exclusively during startup. An error during loading disables the plugin. It is not possible to start a new plugin or modify an existing one while Traefik is running.

Once loaded, middleware plugins behave exactly like statically compiled middlewares. Instantiation and behavior are driven by the dynamic configuration.

Required exports: `type Config struct`, `func CreateConfig() *Config`, `func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error)`.

The page does not mention `Close`, `Stop`, or a reload / teardown hook.
