from functools import lru_cache, cache
import functools


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def make_a():
    return A()


def make_b():
    return B()


@lru_cache
def make_a_cached():
    return A()


@functools.lru_cache
def make_a_ft():
    return A()


@cache
def make_a_cache():
    return A()


def use_direct() -> int:
    return make_a().run() + make_b().run()


def use_assign() -> int:
    a = make_a()
    b = make_b()
    return a.run() + b.run()


def use_cached() -> int:
    return make_a_cached().run() + make_b().run()


def use_functools() -> int:
    return make_a_ft().run() + make_b().run()


def use_cache() -> int:
    return make_a_cache().run() + make_b().run()


def use_preserves_b() -> int:
    return make_b().run()
