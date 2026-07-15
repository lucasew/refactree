const { helper, stay } = require('./helper.cjs');

function use() {
  return helper() + stay();
}

module.exports = { use };
