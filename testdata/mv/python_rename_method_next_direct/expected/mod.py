class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_next_iter_direct(items: list[A], others: list[B]) -> int:
    return next(iter(items)).execute() + next(iter(others)).run()


def use_next_direct(items: list[A], others: list[B]) -> int:
    return next(items).execute() + next(others).run()


def use_next_reversed(items: list[A], others: list[B]) -> int:
    return next(reversed(items)).execute() + next(reversed(others)).run()


def use_next_filter(items: list[A], others: list[B]) -> int:
    return next(filter(None, items)).execute() + next(filter(None, others)).run()


def use_next_list(items: list[A], others: list[B]) -> int:
    return next(iter(list(items))).execute() + next(iter(list(others))).run()


def use_next_assign(items: list[A], others: list[B]) -> int:
    xa = next(iter(items))
    xb = next(others)
    return xa.execute() + xb.run()
