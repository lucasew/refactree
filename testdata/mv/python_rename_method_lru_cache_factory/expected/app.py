from functools import lru_cache, cache
import functools


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@lru_cache
def make_a() -> A:
    return A()


@lru_cache
def make_b() -> B:
    return B()


@functools.lru_cache
def make_a2() -> A:
    return A()


@cache
def make_a3() -> A:
    return A()


def make_plain() -> A:
    return A()


def make_plain_b() -> B:
    return B()


def use_cached() -> int:
    return make_a().execute() + make_b().run()


def use_cached_assign() -> int:
    a = make_a()
    b = make_b()
    return a.execute() + b.run()


def use_functools_cached() -> int:
    return make_a2().execute() + make_b().run()


def use_cache() -> int:
    return make_a3().execute() + make_b().run()


def use_plain() -> int:
    return make_plain().execute() + make_plain_b().run()


def use_plain_assign() -> int:
    a = make_plain()
    b = make_plain_b()
    return a.execute() + b.run()


def use_preserves_b() -> int:
    return make_b().run()
