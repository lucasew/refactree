function helper() {
  return 1;
}
export { helper };
export { helper as h };
export default helper;
const n = helper();
