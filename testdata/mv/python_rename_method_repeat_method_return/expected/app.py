from itertools import repeat
import itertools


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    item: A

    def __init__(self, item: A):
        self.item = item

    def get(self) -> A:
        return self.item


class BoxB:
    item: B

    def __init__(self, item: B):
        self.item = item

    def get(self) -> B:
        return self.item


def use_for(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for xa in repeat(ba.get()):
        n += xa.execute()
        break
    for xb in itertools.repeat(bb.get()):
        n += xb.run()
        break
    return n


def use_for_times(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for xa in repeat(ba.get(), 2):
        n += xa.execute()
    for xb in itertools.repeat(bb.get(), 2):
        n += xb.run()
    return n


def use_next(ba: BoxA, bb: BoxB) -> int:
    return next(repeat(ba.get())).execute() + next(itertools.repeat(bb.get())).run()


def use_assign(ba: BoxA, bb: BoxB) -> int:
    xa = next(repeat(ba.get()))
    xb = next(itertools.repeat(bb.get()))
    return xa.execute() + xb.run()


def use_bind(ba: BoxA, bb: BoxB) -> int:
    ita = repeat(ba.get())
    itb = itertools.repeat(bb.get())
    return next(ita).execute() + next(itb).run()


def use_for_bind(ba: BoxA, bb: BoxB) -> int:
    ita = repeat(ba.get())
    n = 0
    for xa in ita:
        n += xa.execute()
        break
    itb = itertools.repeat(bb.get())
    for xb in itb:
        n += xb.run()
        break
    return n


def use_list_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        list(repeat(ba.get(), 1))[0].execute()
        + list(itertools.repeat(bb.get(), 1))[0].run()
    )


def use_list_for(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for xa in list(repeat(ba.get(), 1)):
        n += xa.execute()
    for xb in list(itertools.repeat(bb.get(), 1)):
        n += xb.run()
    return n


def use_preserves_b(bb: BoxB) -> int:
    n = 0
    for xb in repeat(bb.get()):
        n += xb.run()
        break
    return n + next(repeat(bb.get())).run()
