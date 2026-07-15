public record Box(int assist, int stay) {
  public int use() {
    return assist() + stay();
  }
}
