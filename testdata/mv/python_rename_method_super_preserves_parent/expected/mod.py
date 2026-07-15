class Base:
    def helper(self):
        return 1


class Child(Base):
    def assist(self):
        return super().helper() + 1
