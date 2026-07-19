class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_items(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    n = 0
    for k, ga in da.items():
        n += ga[0].execute()
    for k, gb in db.items():
        n += gb[0].run()
    return n


def use_preserves_b(db: dict[str, list[B]]) -> int:
    n = 0
    for k, gb in db.items():
        n += gb[0].run()
    return n
