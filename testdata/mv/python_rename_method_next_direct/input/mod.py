class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_next_iter_direct(items: list[A], others: list[B]) -> int:
    return next(iter(items)).run() + next(iter(others)).run()


def use_next_direct(items: list[A], others: list[B]) -> int:
    return next(items).run() + next(others).run()


def use_next_reversed(items: list[A], others: list[B]) -> int:
    return next(reversed(items)).run() + next(reversed(others)).run()


def use_next_filter(items: list[A], others: list[B]) -> int:
    return next(filter(None, items)).run() + next(filter(None, others)).run()


def use_next_list(items: list[A], others: list[B]) -> int:
    return next(iter(list(items))).run() + next(iter(list(others))).run()


def use_next_assign(items: list[A], others: list[B]) -> int:
    xa = next(iter(items))
    xb = next(others)
    return xa.run() + xb.run()
