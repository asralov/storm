# Third-party license provenance

The release build gathers full license and notice files from installed production npm packages (recursively, including vendored dependencies), Go module sources, and the Go runtime/vendor sources into `THIRD-PARTY-NOTICES.txt` in the release directory. The generated file is not committed; missing license text fails packaging.

`victory-37.3.6-LICENSE.txt` is retained because that npm package's root MIT license is omitted from its published directory, while its vendored dependency licenses are present. Source: https://raw.githubusercontent.com/FormidableLabs/victory/v37.3.6/LICENSE.txt (retrieved during Phase 8A). Update this pinned notice if victory-vendor changes; review transitive notices before distribution. This inventory is not a legal opinion or substitute for license-compliance review.

Packaging now fails on missing production dependency directories (including optional entries pending explicit platform review), installed name/version mismatches, missing/empty/invalid UTF-8 license text, or a Victory version other than the reviewed 37.3.6. Build-only npm entries remain excluded. Run `node --test scripts/package-release.test.mjs` to exercise these guards. The current Windows production dependency set is fully installed; no platform exclusion was needed.
