from operator import itemgetter
import operator


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_itemgetter(items: list[A]):
    a = itemgetter(0)(items)
    a.execute()


def use_itemgetter_b(items: list[B]):
    b = itemgetter(0)(items)
    b.run()


def use_operator_itemgetter(items: list[A]):
    a = operator.itemgetter(0)(items)
    a.execute()


def use_operator_itemgetter_b(items: list[B]):
    b = operator.itemgetter(0)(items)
    b.run()


def use_itemgetter_walrus(items: list[A]):
    if (a := itemgetter(0)(items)):
        a.execute()


def use_itemgetter_assigned():
    xs = [A()]
    a = itemgetter(0)(xs)
    a.execute()
    ys = [B()]
    b = itemgetter(0)(ys)
    b.run()


def use_itemgetter_wrapper(items: list[A]):
    a = itemgetter(0)(list(items))
    a.execute()
