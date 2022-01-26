// For a detailed explanation regarding each configuration property, visit:
// https://jestjs.io/docs/en/configuration.html

// https://kulshekhar.github.io/ts-jest/docs/getting-started/presets/
// https://github.com/vuejs/vue-cli/tree/dev/packages/%40vue/cli-plugin-unit-jest/presets

module.exports = {
  preset: 'ts-jest/presets/js-with-babel',
  testEnvironment: 'jsdom',
  moduleFileExtensions: [
    'vue',
    'ts',
    'tsx',
    'js',
    'jsx',
    'json',
  ],
  transform: {
    '^.+\\.vue$': require.resolve('@vue/vue3-jest'),
    '.+\\.(css|styl|less|sass|scss|jpg|jpeg|png|svg|gif|eot|otf|webp|ttf|woff|woff2|mp4|webm|wav|mp3|m4a|aac|oga|avif)$':
    require.resolve('jest-transform-stub'),
  },
  transformIgnorePatterns: ['/node_modules/'],
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/src/$1',
  },
  snapshotSerializers: [
    'jest-serializer-vue',
  ],
  testMatch: [
    '**/tests/unit/**/*.spec.[jt]s?(x)',
    '**/__tests__/*.[jt]s?(x)',
  ],
};
