# Profil utilisateur

La [section profil](readeck-instance://profile) de Readeck vous permet de changer vos informations personnelles, votre mot de passe ainsi que d'autres paramètres.

## Modifier votre profil

Sur la page principale du profil, vous pouvez changer votre nom d'utilisateur, votre adresse e-mail et choisir la langue de l'interface.

## Changer de mot de passe

Sur la page [mot de passe](readeck-instance://profile/password) page, vous pouvez changer le mot de passe que vous utilisez pour vous connecter à Readeck.

## Jetons API

Un jeton API vous permet d'utiliser [l'API Readeck](readeck-instance://docs/api) pour développer les applications de votre choix. Vous pouvez créer et gérer les jetons API sur la page [Jetons API](readeck-instance://profile/tokens) de votre profil.

Vous pouvez limiter les permissions d'accès d'un token donné ainsi que sa durée de validité.

## Mots de passe d'application

Si vous avez besoin de donner accès à votre compte Readeck à un service ou une application, vous ne pouvez pas donner votre nom d'utilisateur et mot de passe ; ça ne fonctionnera pas.

Ce que vous pouvez faire est créer un [mot de passe d'application](readeck-instance://profile/credentials).

Vous pouvez limiter ce qu'un mot 

Vous pouvez limiter les permissions d'accès d'un mot de passe d'application.

Une fois que le mot de passe d'application est créé, vous pouvez l'utiliser pour accéder à [l'API Readeck](readeck-instance://docs/api) ou les services d'exportation.

Reportez-vous à l'aide du [catalogue e-book](./opds.md) pour un exemple.

**Note** : même si vous pouvez accéder à l'API avec un mot de passe d'application, il est préférable d'utiliser les jetons API quand c'est possible.
