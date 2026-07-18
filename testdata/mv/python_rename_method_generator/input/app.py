class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def gen_a():
    yield A()


def gen_b():
    yield B()


async def agen_a():
    a = A()
    yield a


async def agen_b():
    yield B()


def use_next():
    return next(gen_a()).run() + next(gen_b()).run()


def use_next_assign():
    a = next(gen_a())
    b = next(gen_b())
    return a.run() + b.run()


def use_for():
    n = 0
    for a in gen_a():
        n += a.run()
    for b in gen_b():
        n += b.run()
    return n


def use_gen_local():
    ga = gen_a()
    gb = gen_b()
    return next(ga).run() + next(gb).run()


def use_list_wrap():
    return next(list(gen_a())).run() + next(list(gen_b())).run()


def use_preserves_b():
    return next(gen_b()).run()
