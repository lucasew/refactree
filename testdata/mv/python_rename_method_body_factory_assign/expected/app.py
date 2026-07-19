from functools import lru_cache


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def make_a():
    a = A()
    return a


def make_b():
    b = B()
    return b


def make_a_multi():
    a = A()
    if True:
        return a
    return a


@lru_cache
def make_a_cached():
    a = A()
    return a


def use_direct() -> int:
    return make_a().execute() + make_b().run()


def use_assign() -> int:
    a = make_a()
    b = make_b()
    return a.execute() + b.run()


def use_multi() -> int:
    return make_a_multi().execute() + make_b().run()


def use_cached() -> int:
    return make_a_cached().execute() + make_b().run()


def use_preserves_b() -> int:
    return make_b().run()
