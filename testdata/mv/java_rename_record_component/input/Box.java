public record Box(int helper, int stay) {
  public int use() {
    return helper() + stay();
  }
}
