from collections import deque
from queue import Queue


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_popleft_chain(as_: deque[A]):
    as_.popleft().run()


def use_popleft_chain_b(bs: deque[B]):
    bs.popleft().run()


def use_pop_chain(as_: list[A]):
    as_.pop().run()


def use_pop_chain_b(bs: list[B]):
    bs.pop().run()


def use_dict_get_chain(am: dict[str, A]):
    am.get("k").run()


def use_dict_get_chain_b(bm: dict[str, B]):
    bm.get("k").run()


def use_queue_get_chain(qa: Queue[A]):
    qa.get().run()


def use_queue_get_chain_b(qb: Queue[B]):
    qb.get().run()


def use_wrapper_pop_chain(as_: list[A]):
    list(as_).pop().run()


def use_wrapper_pop_chain_b(bs: list[B]):
    list(bs).pop().run()


def use_assign_still_ok(as_: deque[A], qb: Queue[B]):
    a = as_.popleft()
    a.run()
    b = qb.get()
    b.run()
