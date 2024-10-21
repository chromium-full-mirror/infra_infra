# Log-viewer component

A component library that provides UI to display and handle logs.
It was built using [ReactJS](https://react.dev/),
[MUI](https://mui.com/), and [Virtusoo](https://virtuoso.dev/).

## Installation

1. Install the required dependencies:

   ```sh
   npm install react react-dom @mui/material @emotion/react @emotion/styled
   ```

2. Install via npm:

   ```sh
   npm install @chopsui/log-viewer
   ```

**Note:** Ensure that `@chopsui` packages are allowlisted in Skia NPM registry for your project.

## Local development

There are two methods to locally test the component:

1. Testing the component with [Storebook](https://storybook.js.org/).

2. Testing the component's integration with your project.

### Using Storybook

* You can build log-viewer's components and see how look and act
  in isolation using [Storybook](https://storybook.js.org/).

* An example of this works is included under `src/base_component/base_component.stories.tsx`.

### Integrating the component with your project

In order to integrate the component with your project we will use
[npm-link](https://docs.npmjs.com/cli/v9/commands/npm-link).
NPM-link creates a symbolic link for the project in the global NPM repository.

**Set up local development:**

1. At the top directory of `log-viewer` run:

   ```sh
   npm link
   ```

   This will create a link to your local npm library copy in your global node_modules.

2. Then at your UI project's top directory run:

   ```sh
   npm link @chopsui/log-viewer
   ```

3. Ensure that your jest config in your project has the following config:

   ```json
   moduleDirectories: ['<rootDir>/node_modules', 'node_modules']
   ```

   This ensures that the library can work with local development environments and jest.

   This will use the linked version of the log-viewer from your global npm_modules.

**Clean up after local development:**

1. Only run this after you have finished your work and _after_ publishing
   the changes to npm registry.

2. Update the version of `@chopsui/log-viewer` in your package.json
   to the one you have published.

3. At your project's top directory run:

   ```sh
   npm ci @chopsui/log-viewer
   ```

   This will install the version you just published and undo the linking.

This will not remove the link we created in the global `node_modules`,
which allows us to reuse this link later, rather this will only remove
it from the project.

If you want to delete the global link as well, run this in the top
directory of `log-viewer`:

```sh
npm unlink -g
```

If you are not yet ready to use the published version, you can skip step
2 and just run step 3, which reset the version to the one you had before linking.

### Integrating the component with your project (Without Linking)

Alternatively, you can utilize [npm local paths](https://docs.npmjs.com/cli/v10/configuring-npm/package-json#local-paths) to test local changes against your project.

e.g. directly install the library in your project's top level directory:

```
npm install -S /usr/local/google/home/$USER/cr/infra/crdx/packages/log-viewer
```

### Technical guidelines for development

1. All components must be extracted in the top directory, except for sub components.

2. All folders must include an `index.ts` files that exposes the components and files that
   will be exposed as an external API.

3. Ensure that your changes are backwards compatible and ideally also forward compatible,
   if a breaking change has to be introduced, then discuss this with the other team.

4. Versioning:
   a. Use PATCH versions for bugfixes and security updates.
   b. use MINOR versions for new components or features.

5. _After_ making your changes and _before_ subtmitting the CL,
   you **must** run this command to ensure the version is updated in the same CL:

   ```sh
   npm run versionPatch # for patch updates

   npm run versionMinor # for minor updates
   ```

### Publishing the library to npm

1. Only publish the package _after_ you have submitted your changes CL changes.

2. After versioning your package run:

   ```sh
   npm publish
   ```
