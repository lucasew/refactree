const m = require('./mod.js');

function use() {
  return m.helper() + m.stay();
}

module.exports = { use };
