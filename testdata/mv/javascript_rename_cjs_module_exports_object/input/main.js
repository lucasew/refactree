const h = require('./helper.js');

function use() {
  return h.helper() + h.stay();
}

module.exports = { use };
