const { assist, stay } = require('./helper.cjs');

function use() {
  return assist() + stay();
}

module.exports = { use };
