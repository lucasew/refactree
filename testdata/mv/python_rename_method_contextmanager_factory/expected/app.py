from contextlib import contextmanager, asynccontextmanager
import contextlib


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@contextmanager
def make_a():
    yield A()


@contextmanager
def make_b():
    yield B()


@contextlib.contextmanager
def make_a2():
    a = A()
    yield a


@contextmanager
def make_b2():
    b = B()
    yield b


@asynccontextmanager
async def make_a_async():
    yield A()


@asynccontextmanager
async def make_b_async():
    yield B()


def use_with() -> int:
    with make_a() as a:
        with make_b() as b:
            return a.execute() + b.run()


def use_with2() -> int:
    with make_a2() as a:
        return a.execute()


def use_with_b2() -> int:
    with make_b2() as b:
        return b.run()


async def use_async() -> int:
    async with make_a_async() as a:
        async with make_b_async() as b:
            return a.execute() + b.run()


def use_preserves_b() -> int:
    with make_b() as b:
        return b.run()
