from collections import Counter
import collections


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_counter_update():
    ca = Counter()
    cb = Counter()
    ca.update([A()])
    cb.update([B()])
    n = 0
    for a in ca:
        n += a.run()
    for b in cb:
        n += b.run()
    return n


def use_counter_update_set():
    ca = Counter()
    cb = Counter()
    ca.update({A()})
    cb.update({B()})
    n = 0
    for a in ca:
        n += a.run()
    for b in cb:
        n += b.run()
    return n


def use_counter_most_common():
    ca = Counter([A()])
    cb = Counter([B()])
    n = 0
    for a, _c in ca.most_common():
        n += a.run()
    for b, _c in cb.most_common():
        n += b.run()
    return n


def use_counter_most_common_assign():
    ca = Counter([A()])
    cb = Counter([B()])
    pairs_a = ca.most_common()
    pairs_b = cb.most_common()
    n = 0
    for a, _c in pairs_a:
        n += a.run()
    for b, _c in pairs_b:
        n += b.run()
    return n


def use_collections_counter_update():
    ca = collections.Counter()
    cb = collections.Counter()
    ca.update([A()])
    cb.update([B()])
    n = 0
    for a in ca:
        n += a.run()
    for b in cb:
        n += b.run()
    return n


def use_preserves_b():
    cb = Counter()
    cb.update([B()])
    n = 0
    for b in cb:
        n += b.run()
    return n
