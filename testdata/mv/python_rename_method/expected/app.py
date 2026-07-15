class Box:
    def fetch_value(self):
        return 1

    def twice(self):
        return self.fetch_value() + Box.fetch_value(self)


def use(b: Box):
    return b.fetch_value()


def make():
    box = Box()
    return box.fetch_value()
