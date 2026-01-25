# Changelog

## [1.5.1](https://github.com/PascaleBeier/hitkeep/compare/v1.5.0...v1.5.1) (2026-01-25)


### Bug Fixes

* **frontend:** Wrong goal % denominator ([f3e7b6f](https://github.com/PascaleBeier/hitkeep/commit/f3e7b6f23fb6c42b80529a8ab8f6c091f28ffdfb))

## [1.5.0](https://github.com/PascaleBeier/hitkeep/compare/v1.4.0...v1.5.0) (2026-01-24)

### Features

- Add Dashboard filters ([b4b25aa](https://github.com/PascaleBeier/hitkeep/commit/b4b25aa728bb8f44ce0404d7e22d9839cefe77a1))
- Add share service ([b7f6572](https://github.com/PascaleBeier/hitkeep/commit/b7f65720fa43cb50aa96da700ae34a978967de75))

## [1.4.0](https://github.com/PascaleBeier/hitkeep/compare/v1.3.0...v1.4.0) (2025-11-24)

### Features

- **api:** Introduce Change Password Route ([2af6260](https://github.com/PascaleBeier/hitkeep/commit/2af6260eb48fc8c776fa592746f9001741ac77ab))
- **frontend:** Introduce Settings Route with Change Password Components ([702c82e](https://github.com/PascaleBeier/hitkeep/commit/702c82e71a43e32f3f1c97b9f76a6363af8f26ce))

### Bug Fixes

- **frontend:** Add settings component and fix mobile drawer issues ([c40a3c8](https://github.com/PascaleBeier/hitkeep/commit/c40a3c8bd2a4c9abcc0228a62b59d825cfe2daa4))

## [1.3.0](https://github.com/PascaleBeier/hitkeep/compare/v1.2.0...v1.3.0) (2025-11-23)

### Features

- **api:** Add Password Reset ([76a2b09](https://github.com/PascaleBeier/hitkeep/commit/76a2b09e027de508b5c8bd9ea884668bbd774ce4))
- **api:** Introduce Mailer integration ([73339b8](https://github.com/PascaleBeier/hitkeep/commit/73339b87bf19221e0138aa4373b8746165dc9b5b))
- **frontend:** Add Password Reset ([0f1e092](https://github.com/PascaleBeier/hitkeep/commit/0f1e092f78bb5cb7825cac8e79e48cf809265ee1))

## [1.2.0](https://github.com/PascaleBeier/hitkeep/compare/v1.1.1...v1.2.0) (2025-11-23)

### Features

- **api:** Add Live visitors metric ([42268ec](https://github.com/PascaleBeier/hitkeep/commit/42268ec405d33653092b65eb8d0e6d52741078ba))
- **api:** add top pages, referrers, and device breakdown to analytics stats ([50deb0b](https://github.com/PascaleBeier/hitkeep/commit/50deb0bc4b6101d87213e728e144de9254b46b0a))
- **frontend:** Add Live visitors metric ([a3c6003](https://github.com/PascaleBeier/hitkeep/commit/a3c6003709fd2e7bc316f8a897a1ca2eefceeb5f))
- **frontend:** add top pages, referrers, and device breakdown to analytics stats ([1df264c](https://github.com/PascaleBeier/hitkeep/commit/1df264c437cc762ebd10ba52568b307b4153edad))

### Bug Fixes

- **api:** Don't prepend www when resolving favicons so sites not listening on www can resolve ([748960f](https://github.com/PascaleBeier/hitkeep/commit/748960fa03be0dc006cfddd869c1c77a7e72a840))
- **api:** Properly close upstream favicon response ([552b114](https://github.com/PascaleBeier/hitkeep/commit/552b1149401aa95e0b80372ef0900d1f5c4b5fef))
- **dx:** Set default DB path to default docker volume for better DX without requiring a breaking change ([8458ddd](https://github.com/PascaleBeier/hitkeep/commit/8458ddd1ea1d285ea32532dd36687de8fd7abe26))
- **dx:** UPDATE README and docker-compose example for new db default path ([2537d62](https://github.com/PascaleBeier/hitkeep/commit/2537d626fb9dd1ab5646a5249147af7de222b71d))

## [1.1.1](https://github.com/PascaleBeier/hitkeep/compare/v1.1.0...v1.1.1) (2025-11-22)

### Bug Fixes

- **deps:** Bump gomod ([70ae350](https://github.com/PascaleBeier/hitkeep/commit/70ae350128b5614ebec5b2e04b56d752a526c193))

## [1.1.0](https://github.com/PascaleBeier/hitkeep/compare/v1.0.0...v1.1.0) (2025-11-21)

### Features

- **ui:** add date range selector and session metrics to dashboard ([89b61f9](https://github.com/PascaleBeier/hitkeep/commit/89b61f9ce69d1c7aa76b31d0baa284a1e598ad51))
- **ui:** Add more UI components for logout, dark mode, make everything a bit easier on the eyes ([b7814c5](https://github.com/PascaleBeier/hitkeep/commit/b7814c5f2efeef159fb4fb7c0fbe04d127b3f16a))
- **ui:** Allow all kinds of date ranges ([5481272](https://github.com/PascaleBeier/hitkeep/commit/54812728af2adb5daa8bc0e6dbde26505d7edc15))

### Bug Fixes

- **db:** normalize time boundaries for OLAP chart query to fix zero-data bug ([dab4e45](https://github.com/PascaleBeier/hitkeep/commit/dab4e45b6b67b19365997eb8e9bd6ae8dc52acc1))

## 1.0.0 (2025-11-21)

### Features

- **backend:** Add configurable Ratelimiter for endpoints ([925c356](https://github.com/PascaleBeier/hitkeep/commit/925c356bb7dbfaeeebf58c8c0a18619646cec149))
