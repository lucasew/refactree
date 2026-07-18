from operator import itemgetter
import operator


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_itemgetter_direct(items: list[A]) -> int:
    return itemgetter(0)(items).run()


def use_operator_itemgetter_direct(items: list[A]) -> int:
    return operator.itemgetter(0)(items).run()


def use_itemgetter_wrapper(items: list[A]) -> int:
    return itemgetter(0)(list(items)).run()


def use_itemgetter_assign(items: list[A]) -> int:
    # assignment path still works (regression)
    a = itemgetter(0)(items)
    return a.run()


def use_b_itemgetter(others: list[B]) -> int:
    return itemgetter(0)(others).run()


def use_b_operator(bobs: list[B]) -> int:
    return operator.itemgetter(0)(bobs).run()
